package gossiper

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/lukasdeloose/Peerster/helpers"
	"github.com/lukasdeloose/Peerster/packets"
	"io/ioutil"
	"os"
)

type File struct {
	name     string
	size     int
	metaFile []byte
	hash     []byte
}

func (gossiper *Gossiper) ShareFile(fileName string) {
	path := "_SharedFiles/" + fileName
	f, err := os.Open(path)
	if !helpers.Check(err) {
		return
	}

	defer f.Close()

	h := sha256.New()

	metaFile := make([]byte, 0)

	numBytes := helpers.ChunkSize
	fileSize := 0
	// Loop until we reach end of the file
	for numBytes == helpers.ChunkSize {
		buffer := make([]byte, helpers.ChunkSize)
		reader := bufio.NewReader(f)
		numBytes, err = reader.Read(buffer)
		if !helpers.Check(err) {
			// If size is exactly 8KiB, EOF
			break
		}
		fileSize = fileSize + numBytes
		h.Write(buffer[:numBytes])
		chunkHash := h.Sum(nil)
		metaFile = append(metaFile, chunkHash...)

		// Save all data in map
		gossiper.fileDataLock.Lock()
		gossiper.fileData[hex.EncodeToString(chunkHash)] = buffer[:numBytes]
		gossiper.fileDataLock.Unlock()
		h.Reset()
	}

	h.Write(metaFile)

	file := &File{
		name:     fileName,
		size:     fileSize,
		metaFile: metaFile,
		hash:     h.Sum(nil),
	}
	gossiper.files[hex.EncodeToString(file.hash)] = file
	fmt.Println("METAFILE", hex.EncodeToString(file.hash))

	txPublish := packets.TxPublish{
		Name:         file.name,
		Size:         int64(file.size),
		MetafileHash: file.hash,
	}

	blockPublish := packets.BlockPublish{
		PrevHash:    [32]byte{},
		Transaction: txPublish,
	}

	if gossiper.hw3ex3 {
		go gossiper.publishHw3ex3(blockPublish, false, 0)
	} else {
		go gossiper.publishHw3ex2(blockPublish)

	}
}

func (gossiper *Gossiper) saveFile(fileName string, data []byte) {
	err := ioutil.WriteFile("_Downloads/"+fileName, data, 0644)
	if helpers.Check(err) {
		fmt.Println("RECONSTRUCTED file", fileName)
	}
}

// Retrieve the data from own memory, returns nil if data not known
func (gossiper *Gossiper) retrieveData(hashValue []byte) []byte {
	// Hash of metaFile
	if data, ok := gossiper.files[hex.EncodeToString(hashValue)]; ok {
		return data.metaFile
	}
	gossiper.fileDataLock.RLock()
	defer gossiper.fileDataLock.RUnlock()
	// Hash of chunk
	if data, ok := gossiper.fileData[hex.EncodeToString(hashValue)]; ok {
		return data
	}
	return nil
}

// Split the MetaFile in slice of different hashes
func (gossiper *Gossiper) getChunkHashes(metaFile []byte) [][]byte {
	hashLength := len(metaFile)
	numberOfHashes := hashLength / helpers.HashSize
	chunkHashes := make([][]byte, 0)
	for i := 0; i < numberOfHashes; i++ {
		chunkHash := metaFile[helpers.HashSize*i : helpers.HashSize*(i+1)]
		chunkHashes = append(chunkHashes, chunkHash)
	}
	return chunkHashes
}
