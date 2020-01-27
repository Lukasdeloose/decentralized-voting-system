import asyncio
import hashlib
import numpy as np
import pytest

from setup import Setup


PEERSTER_ROOT = "/home/thomas/go/src/github.com/lukasdeloose/decentralized-voting-system/project/project"


@pytest.mark.skip(reason="Use wrappers around these async methods")
class Tests:
    @staticmethod
    async def test_private_messages():
        NUM_PEERS = 100

        s = Setup.create_line_setup(PEERSTER_ROOT, NUM_PEERS)

        s.run_all()
        await asyncio.sleep(.25)

        for peerster in s.peersters:
            peerster.send_public_message(f" ")
        await asyncio.sleep(1)

        msg_map = {}
        for i, peerster in enumerate(s.peersters):
            if i - 7 < 0 and not i + 7 >= len(s.peersters):
                other = i + 7
            else:
                other = i - 7

            msg_map[other] = msg_map.get(other, []) + [i]

            peerster.send_private_message(f"Test{i}",
                                          f"testPeer{other}")
        await asyncio.sleep(1)

        await s.stop_all()

        for i, peerster in enumerate(s.peersters):
            others = msg_map.get(i)
            if others is None:
                continue
            for other in others:
                peer = f"testPeer{other}"
                assert peer in peerster.private_messages.keys()
                assert len(peerster.private_messages[peer]) == 1
                assert peerster.private_messages[peer][0] == {"msg": f"Test{other}", "hop-lim": "3"}

    @staticmethod
    async def test_public_messages():
        NUM_PEERS = 100

        s = Setup.create_line_setup(PEERSTER_ROOT, NUM_PEERS)

        s.run_all()
        await asyncio.sleep(.25)

        for i, peerster in enumerate(s.peersters):
            peerster.send_public_message(f"Test{i}")
        await asyncio.sleep(1)

        await s.stop_all()

        for i, peerster in enumerate(s.peersters):
            print(peerster.public_messages)
            for j, other_peerster in enumerate(s.peersters):
                if i == j:
                    continue
                assert peerster.public_messages[f"testPeer{j}"] == {1: f"Test{j}"}

    @staticmethod
    async def test_file_upload():
        NUM_PEERS = 10

        s = Setup.create_line_setup(PEERSTER_ROOT, NUM_PEERS)

        s.run_all()
        await asyncio.sleep(.5)

        # Upload files
        files = []
        for i, peerster in enumerate(s.peersters):
            with open(f"{peerster.root}/_SharedFiles/test{i}.txt", "w+") as f:
                to_file = "12345678" * 1000 + peerster.name + "12345678" * 1000
                files.append(to_file)
                f.write(to_file)
            peerster.upload_file(f"test{i}.txt")
        await asyncio.sleep(.5)

        await s.stop_all()

        # Assert files are uploaded
        for i, peerster in enumerate(s.peersters):
            with open(f"{peerster.root}/_SharedFiles/test{i}.txt", "rb") as f:
                h = Tests._calc_hash(f)[0]
            assert h in peerster.uploaded_files

    @staticmethod
    async def test_file_download():
        NUM_PEERS = 30

        s = Setup.create_line_setup(PEERSTER_ROOT, NUM_PEERS)

        s.run_all()
        await asyncio.sleep(.5)

        # Send route rumors
        for i, peerster in enumerate(s.peersters):
            peerster.send_public_message(f" ")

        # Upload files
        files = []
        for i, peerster in enumerate(s.peersters):
            with open(f"{peerster.root}/_SharedFiles/test{i}.txt", "w+") as f:
                to_file = "12345678" * 1000 + peerster.name + "12345678" * 1000
                files.append(to_file)
                f.write(to_file)
            peerster.upload_file(f"test{i}.txt")
        await asyncio.sleep(10)

        # Don't calculate hash ourselves, but use peerster hashes: test doesn't fail
        # if hashing/chunking is done incorrectly
        metas = []
        for i, peerster in enumerate(s.peersters):
            assert len(peerster.uploaded_files) == 1
            hash = next(iter(peerster.uploaded_files))
            assert len(hash) == 64
            metas.append(hash)

        await asyncio.sleep(10)

        for i, peerster in enumerate(s.peersters):
            other = (i - 7) % len(s.peersters)
            peerster.download_file(f"test{other}_d.txt", metas[other], f"testPeer{other}")

        await asyncio.sleep(10)
        for i, peerster in enumerate(s.peersters):
            assert f"test{(i-7)%len(s.peersters)}_d.txt" in peerster.downloaded_files

        await s.stop_all()

    @staticmethod
    async def _setup_searched_files(s, diff, budget=None):
        # Upload files
        hashes = {"left": {}, "right": {}}
        for i, peerster in enumerate(s.peersters):
            if i - diff >= 0 and i + diff < len(s.peersters):
                with open(f"{peerster.root}/_SharedFiles/test{i}_left.txt", "w+") as f:
                    to_file = "12345678" * 1000 + s.peersters[i - diff].name + peerster.name + "12345678" * 1000
                    f.write(to_file)
                with open(f"{peerster.root}/_SharedFiles/test{i}_left.txt", "rb") as f:
                    hashes["left"][i] = Tests._calc_hash(f)

                with open(f"{peerster.root}/_SharedFiles/test{i}_right.txt", "w+") as f:
                    to_file = "12345678" * 1000 + s.peersters[i + diff].name + peerster.name + "12345678" * 1000
                    f.write(to_file)
                with open(f"{peerster.root}/_SharedFiles/test{i}_right.txt", "rb") as f:
                    hashes["right"][i] = Tests._calc_hash(f)

                s.peersters[i - diff].upload_file(f"test{i}_left.txt")
                s.peersters[i + diff].upload_file(f"test{i}_right.txt")

        # Wait for files to be uploaded and route rumors to have spread
        await asyncio.sleep(10)

        # Perform searches
        for i, peerster in enumerate(s.peersters):
            if i - diff >= 0 and i + diff < len(s.peersters):
                peerster.search([f"test{i}"], budget=budget)
                await asyncio.sleep(2)

        # Wait for searches to complete
        await asyncio.sleep(10)

        return hashes

    @staticmethod
    async def test_search():
        NUM_PEERS = 10
        DIFF = 2
        BUDGET = None

        s = Setup.create_line_setup(PEERSTER_ROOT, NUM_PEERS)

        s.run_all()
        await asyncio.sleep(.5)

        # Send route rumors
        for peerster in s.peersters:
            peerster.send_public_message(f" ")

        hashes = await Tests._setup_searched_files(s, DIFF, budget=BUDGET)

        # Stop all peersters
        await s.stop_all()

        # Assert that searches are correct
        for i, peerster in enumerate(s.peersters):
            if i - DIFF >= 0 and i + DIFF < len(s.peersters):
                assert len(peerster.searches) == 1
                results = peerster.searches[0]
                assert {"file_name": f"test{i}_left.txt",
                        "peer": f"testPeer{i-DIFF}",
                        "meta": hashes["left"][i][0],
                        "chunks": [f"{i}" for i in range(1, hashes["left"][i][1]+1)]} in results
                assert {"file_name": f"test{i}_right.txt",
                        "peer": f"testPeer{i+DIFF}",
                        "meta": hashes["right"][i][0],
                        "chunks": [f"{i}" for i in range(1, hashes["right"][i][1]+1)]} in results

    @staticmethod
    async def test_download_searched_files():
        NUM_PEERS = 10
        DIFF = 2

        s = Setup.create_line_setup(PEERSTER_ROOT, NUM_PEERS)

        s.run_all()
        await asyncio.sleep(.5)

        # Send route rumors
        for peerster in s.peersters:
            peerster.send_public_message(f" ")

        hashes = await Tests._setup_searched_files(s, DIFF)

        # Download files
        for i, peerster in enumerate(s.peersters):
            if i - DIFF >= 0 and i + DIFF < len(s.peersters):
                peerster.download_searched_file(f"test{i}_left_d.txt", hashes["left"][i][0])
                peerster.download_searched_file(f"test{i}_right_d.txt", hashes["right"][i][0])

        # Wait for files to be downloaded
        await asyncio.sleep(15)

        # Stop all peersters
        await s.stop_all()

        for i, peerster in enumerate(s.peersters):
            if i - DIFF >= 0 and i + DIFF < len(s.peersters):
                assert f"test{i}_left_d.txt" in peerster.downloaded_files
                assert f"test{i}_right_d.txt" in peerster.downloaded_files

    @staticmethod
    async def test_confirmed_file():
        NUM_PEERS = 8

        s = Setup.create_line_setup(PEERSTER_ROOT, NUM_PEERS, hw3ex2=True)

        s.run_all()
        await asyncio.sleep(.5)

        # Send route rumors
        for peerster in s.peersters:
            peerster.send_public_message(f" ")

        await asyncio.sleep(3)

        # Upload a file to each node
        hashes = []
        sizes = []
        for i, peerster in enumerate(s.peersters):
            with open(f"{peerster.root}/_SharedFiles/test{i}.txt", "w+") as f:
                to_file = "12345678" * 1000 + peerster.name + "12345678" * 1000
                sizes.append(len(to_file))
                f.write(to_file)
            with open(f"{peerster.root}/_SharedFiles/test{i}.txt", "rb") as f:
                hashes.append(Tests._calc_hash(f)[0])
            peerster.upload_file(f"test{i}.txt")
        await asyncio.sleep(15)

        for i, peerster in enumerate(s.peersters):
            # check that correct file was rebroadcasted
            for re_br in peerster.re_broadcast:
                assert len(re_br["witnesses"]) > NUM_PEERS / 2

            for j, other in enumerate(s.peersters):
                if i == j:
                    continue
                info = {"origin": other.name, "name": f"test{j}.txt", "meta": hashes[j], "id": "3", "size": str(sizes[j])}
                assert info in peerster.confirmed

        # Now try to upload a duplicate file
        for i, peerster in enumerate(s.peersters):
            peerster.upload_file(f"test{len(s.peersters)-i-1}.txt")
        await asyncio.sleep(3)

        await s.stop_all()

        # It should not show up in uploaded files!
        for i, peerster in enumerate(s.peersters):
            assert hashes[-i-1] not in peerster.uploaded_files

    @staticmethod
    def _calc_hash(f):
        chunk_size = 8192
        hs = b""
        num_chunks = 0
        while True:
            bs = f.read(chunk_size)
            if len(bs) == 0:
                break
            hs += hashlib.sha256(bs).digest()
            num_chunks += 1
        return hashlib.sha256(hs).digest().hex(), num_chunks


# -- PyTest Tests -- #
def test_public_messages():
    loop = asyncio.get_event_loop()
    loop.run_until_complete(Tests.test_public_messages())


def test_private_messages():
    loop = asyncio.get_event_loop()
    loop.run_until_complete(Tests.test_private_messages())


@pytest.mark.skip(reason="Not needed in project")
def test_file_upload():
    loop = asyncio.get_event_loop()
    loop.run_until_complete(Tests.test_file_upload())


@pytest.mark.skip(reason="Not needed in project")
def test_file_download():
    loop = asyncio.get_event_loop()
    loop.run_until_complete(Tests.test_file_download())


@pytest.mark.skip(reason="Not needed in project")
def test_search():
    loop = asyncio.get_event_loop()
    loop.run_until_complete(Tests.test_search())


@pytest.mark.skip(reason="Not needed in project")
def test_download_searched_files():
    loop = asyncio.get_event_loop()
    loop.run_until_complete(Tests.test_download_searched_files())


@pytest.mark.skip(reason="Not needed in project")
def test_confirmed_file():
    loop = asyncio.get_event_loop()
    loop.run_until_complete(Tests.test_confirmed_file())
