package gossiper

import (
	"encoding/hex"
	"fmt"
	"github.com/lukasdeloose/Peerster/helpers"
	"github.com/lukasdeloose/Peerster/packets"
	"math"
	"strings"
	"sync"
)

// Uniquely identifies a file, used for fileSearch
type FileIdentifier struct {
	Name         string
	MetaFileHash string
}

type SearchMap struct {
	searchMap map[FileIdentifier]map[string]*packets.SearchResult // Map a file on a map of the user to the respective SearchResult
	mapLock   sync.RWMutex
	searchGui []FileIdentifier
	guiLock   sync.RWMutex
}

func NewSearchMap() *SearchMap {
	return &SearchMap{
		searchMap: make(map[FileIdentifier]map[string]*packets.SearchResult),
		searchGui: make([]FileIdentifier, 0),
	}
}

func (sm *SearchMap) add(id FileIdentifier, user string, result *packets.SearchResult) {
	sm.mapLock.Lock()
	if _, ok := sm.searchMap[id]; !ok {
		sm.searchMap[id] = make(map[string]*packets.SearchResult)
		// File not present yet, append to gui
		sm.guiLock.Lock()
		sm.searchGui = append(sm.searchGui, id)
		sm.guiLock.Unlock()
	}
	sm.searchMap[id][user] = result
	sm.mapLock.Unlock()
}

func (sm *SearchMap) GetGui() []FileIdentifier {
	// Slice is passed by value, thread-safe (only appends)
	return sm.searchGui
}

// Primitive function to get peers who have the full file
func (sm *SearchMap) getDestPeers(name, hash string) []string {
	id := FileIdentifier{
		Name:         name,
		MetaFileHash: hash,
	}
	destPeers := make([]string, 0)
	sm.mapLock.RLock()
	for peer, result := range sm.searchMap[id] {
		if uint64(len(result.ChunkMap)) == result.ChunkCount {
			destPeers = append(destPeers, peer)
		}
	}
	sm.mapLock.RUnlock()
	return destPeers
}

func (gossiper *Gossiper) searchFile(keywords []string) ([]*packets.SearchResult, bool) {
	results := make([]*packets.SearchResult, 0)
	gossiper.filesLock.RLock()
	defer gossiper.filesLock.RUnlock()
	for _, file := range gossiper.files {
		for _, keyword := range keywords {
			if strings.Contains(file.name, keyword) {
				chunkMap := gossiper.getChunkMap(file.metaFile)
				result := &packets.SearchResult{
					FileName:     file.name,
					MetafileHash: file.hash,
					ChunkMap:     chunkMap,
					ChunkCount:   uint64(math.Ceil(float64(file.size) / float64(helpers.ChunkSize))),
				}
				if gossiper.debug {
					fmt.Println("result name", result.FileName, "result hash", result.MetafileHash, "result chunkMap", result.ChunkMap, "result chunkcount", result.ChunkCount)
				}
				results = append(results, result)
				break
			}
		}
	}
	if gossiper.debug {
		fmt.Println("results: ", results)
	}
	if len(results) > 0 {
		return results, true
	}
	return nil, false
}

func (gossiper *Gossiper) getChunkMap(metaFile []byte) []uint64 {
	chunkHashes := gossiper.getChunkHashes(metaFile)
	chunkMap := make([]uint64, 0)
	gossiper.fileDataLock.RLock()
	defer gossiper.fileDataLock.RUnlock()
	for i, chunk := range chunkHashes {
		if _, found := gossiper.fileData[hex.EncodeToString(chunk)]; found {
			chunkMap = append(chunkMap, uint64(i+1))
		}
	}
	return chunkMap
}

// Processes a searchResult
// Returns number of full matches and fileID
func (gossiper *Gossiper) processSearchResult(result *packets.SearchResult, origin string) int {
	matches := 0
	fmt.Print("FOUND match ", result.FileName, " at ", origin, " metafile=", hex.EncodeToString(result.MetafileHash),
		" chunks=", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(result.ChunkMap)), ","), "[]"), "\n")

	// All chunks present, full match
	if len(result.ChunkMap) == int(result.ChunkCount) {
		matches++
		if gossiper.debug {
			fmt.Println("Full match for", result.FileName)
		}
	}
	fileIdentifier := FileIdentifier{
		Name:         result.FileName,
		MetaFileHash: hex.EncodeToString(result.MetafileHash),
	}
	gossiper.searchMap.add(fileIdentifier, origin, result)
	return matches
}

func (gossiper *Gossiper) downloadSearchedFile(result *packets.SearchResult, origin string) {
	message := &packets.Message{
		Text:        "",
		Destination: &origin,
		File:        &result.FileName,
		Request:     &result.MetafileHash,
		KeyWords:    "",
		Budget:      0,
	}
	gossiper.HandleClientRequest(message)
}
