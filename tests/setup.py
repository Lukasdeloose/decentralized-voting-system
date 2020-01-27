import numpy as np
import os
import sys
import subprocess as sp
import re
from time import sleep
from datetime import datetime
from threading import Lock
import asyncio


class Peerster:

    def __init__(self, exec_path, uiport, gossip_addr, name, peers, anti_entropy, rtimer, hw3ex2=False, N=None):
        self.exec_path = exec_path
        self.uiport = uiport
        self.gossip_addr = gossip_addr
        self.name = name
        self.peers = peers

        self._start_cmd = [exec_path,
                           "-UIPort", uiport,
                           "-gossipAddr", gossip_addr,
                           "-name", name,
                           "-peers", ",".join(peers),
                           "-debug",
                           "-antiEntropy", str(anti_entropy),
                           "-rtimer", str(rtimer)]

        if hw3ex2:
            self._start_cmd += ["-hw3ex2"]
            if N is None:
                raise AttributeError("N must be provided in hw3ex2 mode!")
            self._start_cmd += ["-N", str(N)]

        self._client_cmd = [os.path.join(os.path.dirname(exec_path), "client", "client"),
                            "-UIPort", self.uiport]
        self.mutex = Lock()

        self.peers = []
        self.dsdv = {}

        self.public_messages = {}
        self.public_messages_next_id = {}
        self.private_messages = {}
        self.uploaded_files = set()
        self.downloaded_files = set()
        self.client_messages = []
        self.in_sync_with = set()

        self.searches = []
        self.current_search = []

        self.metafiles = []
        self.downloading = {}

        self.unconfirmed = []
        self.sending_ack = []
        self.re_broadcast = []
        self.confirmed = []

        self.kill_sig = asyncio.Event()

    def _update_peers(self, peers):
        with self.mutex:
            self.peers = peers

    def _update_dsdv(self, peer, addr):
        with self.mutex:
            self.dsdv[peer] = addr

    def _update_public_messages(self, origin, id, content):
        with self.mutex:
            if self.public_messages.get(origin) is None:
                self.public_messages[origin] = {}
            self.public_messages[origin][int(id)] = content
            next_id = int(self.public_messages_next_id.get(origin, "1"))
            while self.public_messages[origin].get(next_id) is not None:
                next_id += 1
                self.public_messages_next_id[origin] = next_id

    def _update_in_sync_with(self, peer):
        with self.mutex:
            self.in_sync_with.add(peer)

    def _update_private_messages(self, msg, origin, hop_limit):
        with self.mutex:
            if self.private_messages.get(origin) is None:
                self.private_messages[origin] = []
            self.private_messages[origin].append({"msg": msg, "hop-lim": hop_limit})

    def _update_uploaded_files(self, meta):
        with self.mutex:
            self.uploaded_files.add(meta)

    def _update_downloaded_files(self, filename):
        with self.mutex:
            self.downloaded_files.add(filename)

    def _update_searches(self, file_name, peer, meta, chunks):
        with self.mutex:
            self.current_search.append({"file_name": file_name, "peer": peer, "meta": meta, "chunks": chunks.strip().split(",")})

    def _finish_search(self):
        with self.mutex:
            assert len(self.current_search) > 0  # search should not be finished if there aren't any matches
            if self.current_search not in self.searches:
                self.searches.append(self.current_search.copy())
            self.current_search = []

    def _update_unconfirmed(self, origin, id, name, size, meta):
        with self.mutex:
            info = {"origin": origin, "id": id, "name": name, "size": size, "meta": meta}
            if info not in self.unconfirmed:
                self.unconfirmed.append(info)

    def _update_ack_sent(self, origin, id):
        with self.mutex:
            self.sending_ack.append({"origin": origin, "id": id})

    def _update_re_broadcast(self, id, witnesses):
        with self.mutex:
            self.re_broadcast.append({"id": id, "witnesses": witnesses.strip().split(",")})

    def _update_confirmed(self, origin, id, name, size, meta):
        with self.mutex:
            info = {"origin": origin, "id": id, "name": name, "size": size, "meta": meta}
            if info not in self.confirmed:
                self.confirmed.append(info)

    async def run(self):
        # start child process
        # NOTE: universal_newlines parameter is not supported
        self._killed = False
        process = await asyncio.create_subprocess_exec(*self._start_cmd,
                                                       stdout=asyncio.subprocess.PIPE,
                                                       cwd=os.path.dirname(self.exec_path))

        # Make sure the directory for the output files exists
        if not os.path.isdir("out"):
            os.makedirs("out")

        # read line (sequence of bytes ending with b'\n') asynchronously
        with open(os.path.join("out", f"{self.name}.out"), "w+") as f:
            while True:
                try:
                    line = await asyncio.wait_for(process.stdout.readline(), .1)
                except asyncio.TimeoutError:
                    if self.kill_sig.is_set():
                        process.kill()
                        return await process.wait()
                else:
                    line = line.decode(sys.stdout.encoding).strip()
                    if not line:  # EOF
                        break
                    else:
                        f.write(f"[{datetime.now()}] {line}\n")
                        if self._starts_with(line, "PEERS"):
                            self._update_peers(line[len("PEERS "):].split(","))
                        elif self._starts_with(line, "DSDV"):
                            try:
                                origin, addr = line.split()[1:]
                                self._update_dsdv(origin, addr)
                            except:
                                pass

                        elif self._starts_with(line, "IN SYNC WITH"):
                            self._update_in_sync_with(line[len("IN SYNC WITH "):])
                        elif self._starts_with(line, "RUMOR"):
                            m = re.match(r"RUMOR origin ([a-zA-Z0-9]+) from ([a-zA-Z0-9.:]+) ID ([0-9]+) contents ?(.*)$", line)
                            self._update_public_messages(m.groups()[0], m.groups()[2], m.groups()[3])
                        elif self._starts_with(line, "PRIVATE"):
                            m = re.match(r"PRIVATE origin ([a-zA-Z0-9]+) hop-limit ([0-9]+) contents ?(.*)$",
                                         line)
                            self._update_private_messages(m.groups()[2], m.groups()[0], m.groups()[1])
                        elif self._starts_with(line, "CLIENT MESSAGE"):
                            line = line[len("CLIENT MESSAGE "):].strip()
                            if " dest " in line:
                                msg, dest = line.split(" dest ")
                                if dest == self.name:
                                    self._update_private_messages(msg, dest, hop_limit=10)
                            else:
                                self._update_public_messages(self.name,
                                                             len(self.public_messages.get(self.name, {}).keys()) + 1,
                                                             line)

                        elif self._starts_with(line, "METAFILE"):
                            meta = line[len("METAFILE "):]
                            self._update_uploaded_files(meta.strip())

                        elif self._starts_with(line, "RECONSTRUCTED file"):
                            name = line[len("RECONSTRUCTED file "):]
                            self._update_downloaded_files(name.strip())

                        elif self._starts_with(line, "FOUND match"):
                            m = re.match(r"FOUND match ([a-zA-Z0-9._]+) at ([a-zA-Z0-9]+) metafile=([a-f0-9]+) chunks=([0-9,]*)$", line)
                            self._update_searches(m.groups()[0], m.groups()[1], m.groups()[2], m.groups()[3])

                        elif self._starts_with(line, "SEARCH FINISHED"):
                            self._finish_search()

                        elif self._starts_with(line, "UNCONFIRMED GOSSIP"):
                            m = re.match(r"UNCONFIRMED GOSSIP origin ([a-zA-Z0-9]+) ID ([0-9]+) filename ([a-zA-Z0-9._]+) size ([0-9]+) metahash ([a-f0-9]+)$", line)
                            self._update_unconfirmed(m.groups()[0], m.groups()[1], m.groups()[2], m.groups()[3], m.groups()[4])

                        elif self._starts_with(line, "SENDING ACK"):
                            m = re.match(r"SENDING ACK origin ([a-zA-Z0-9]+) ID ([0-9]+)$", line)
                            self._update_ack_sent(m.groups()[0], m.groups()[1])

                        elif self._starts_with(line, "RE-BROADCAST"):
                            m = re.match(r"RE-BROADCAST ID ([0-9]+) WITNESSES ([a-zA-Z0-9,]+)$", line)
                            self._update_re_broadcast(m.groups()[0], m.groups()[1])

                        elif self._starts_with(line, "CONFIRMED GOSSIP"):
                            m = re.match(r"CONFIRMED GOSSIP origin ([a-zA-Z0-9]+) ID ([0-9]+) filename ([a-zA-Z0-9._]+) size ([0-9]+) metahash ([a-f0-9]+)$", line)
                            self._update_confirmed(m.groups()[0], m.groups()[1], m.groups()[2], m.groups()[3], m.groups()[4])

            process.kill()
            return await process.wait()

    async def kill(self):
        self.kill_sig.set()

    def send_public_message(self, msg):
        proc = sp.run(self._client_cmd + ["-msg", msg], stdout=sp.PIPE)
        if proc.returncode != 0:
            raise RuntimeError(f"Could not send message '{msg}' with client {self.name}")

    def send_private_message(self, msg, to):
        proc = sp.run(self._client_cmd + ["-msg", msg] + ["-dest", to])
        if proc.returncode != 0:
            raise RuntimeError(f"Could not send private message '{msg}' with client {self.name}")

    def upload_file(self, file_name):
        proc = sp.run(self._client_cmd + ["-file", file_name])
        if proc.returncode != 0:
            raise RuntimeError(f"Could not upload file '{file_name}' with client {self.name}")

    def download_file(self, file_name, metadata, peer):
        proc = sp.run(self._client_cmd + ["-file", file_name, "-request", metadata, "-dest", peer])
        if proc.returncode != 0:
            raise RuntimeError(f"Could not download file '{file_name}' with client {self.name}")

    def search(self, keywords, budget=None):
        args = ["-keywords", ",".join(keywords)]
        if budget is not None:
            args += ["-budget", str(budget)]

        proc = sp.run(self._client_cmd + args)
        if proc.returncode != 0:
            raise RuntimeError(f"Could not perform search '{','.join(keywords)}' with client {self.name}")

    def download_searched_file(self, file_name, request):
        proc = sp.run(self._client_cmd + ["-file", file_name, "-request", request])
        if proc.returncode != 0:
            raise RuntimeError(f"Could not download searched file '{file_name}' with client {self.name}")

    def _starts_with(self, string, start):
        # Helper method to determine if a received line start with a certain string
        return string[:len(start)] == start


class Setup:

    def __init__(self, root, num_peers, conn_matrix, anti_entropy=0, rtimer=0, hw3ex2=False):
        # create connectivity matrix
        self.conn_matrix = conn_matrix

        # create peer lists
        peers = []
        for row in range(self.conn_matrix.shape[0]):
            p_ = []
            for col in range(self.conn_matrix.shape[1]):
                if self.conn_matrix[row][col]:
                    p_.append(f"127.0.0.1:{5000 + col}")
            peers.append(p_)

        # create peersters
        self.peersters = []
        for i in range(num_peers):
            if isinstance(root, list):
                r = root[i % len(root)]
            else:
                r = root
            self.peersters.append(Peerster(r,
                                           uiport=f"{8080+i}",
                                           gossip_addr=f"127.0.0.1:{5000+i}",
                                           name=f"testPeer{i}",
                                           peers=peers[i],
                                           anti_entropy=anti_entropy,
                                           rtimer=rtimer,
                                           hw3ex2=hw3ex2,
                                           N=num_peers))

    @staticmethod
    def create_line_setup(root, num_peers, anti_entropy=0, rtimer=0, hw3ex2=False):
        conn_matrix = np.zeros((num_peers, num_peers))
        np.fill_diagonal(conn_matrix[:, 1:], 1)
        np.fill_diagonal(conn_matrix[1:, :], 1)

        return Setup(root, num_peers, conn_matrix, anti_entropy=anti_entropy, rtimer=rtimer, hw3ex2=hw3ex2)

    def run_all(self):
        loop = asyncio.get_event_loop()
        self.tasks = []
        for peerster in self.peersters:
            self.tasks.append(loop.create_task(peerster.run()))

    async def stop_all(self):
        for peerster in self.peersters:
            await peerster.kill()
        await asyncio.gather(*self.tasks)
