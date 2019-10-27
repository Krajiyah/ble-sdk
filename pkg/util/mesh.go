package util

import (
	"github.com/RyanCarrier/dijkstra"
)

func abs(x int) int {
	if x >= 0 {
		return x
	}
	return x * -1
}

func rememberHash(s string, mem map[int]string, reverseMem map[string]int, numNodes int) (int, int) {
	var h int
	var ok bool
	if h, ok = reverseMem[s]; !ok {
		h = numNodes
	}
	mem[h] = s
	reverseMem[s] = h
	numNodes = numNodes + 1
	return h, numNodes
}

// ShortestPathToServer tells forwarder which path to take to forward packets to server
func ShortestPathToServer(forwarderAddr string, serverAddr string, rssiMap map[string]map[string]int) ([]string, error) {
	graph := dijkstra.NewGraph()
	hashMem := map[int]string{}
	reverseHashMem := map[string]int{}
	numNodes := 0
	var dst int
	var src int
	for addr := range rssiMap {
		src, numNodes = rememberHash(addr, hashMem, reverseHashMem, numNodes)
		graph.AddVertex(src)
		for dstAddr := range rssiMap[addr] {
			dst, numNodes = rememberHash(dstAddr, hashMem, reverseHashMem, numNodes)
			graph.AddArc(src, dst, int64(abs(rssiMap[addr][dstAddr])))
		}
	}
	src, numNodes = rememberHash(forwarderAddr, hashMem, reverseHashMem, numNodes)
	dst, numNodes = rememberHash(serverAddr, hashMem, reverseHashMem, numNodes)
	_, err := graph.GetVertex(src)
	if err != nil {
		return nil, err
	}
	_, err = graph.GetVertex(dst)
	if err != nil {
		return nil, err
	}
	shortest, err := graph.Shortest(src, dst)
	if err != nil {
		return nil, err
	}
	ret := make([]string, len(shortest.Path))
	for ele := range shortest.Path {
		ret = append(ret, hashMem[ele])
	}
	return ret, nil
}
