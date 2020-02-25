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

func rememberHash(s string, mem map[int]string, reverseMem map[string]int, numNodes int) (int, bool, int) {
	var h int
	var ok bool
	if h, ok = reverseMem[s]; !ok {
		h = numNodes
		numNodes = numNodes + 1
	}
	mem[h] = s
	reverseMem[s] = h
	return h, !ok, numNodes
}

// ShortestPath tells forwarder which path to take to forward packets to server
func ShortestPath(rssiMap map[string]map[string]int, forwarderAddr string, serverAddr string) ([]string, error) {
	graph := dijkstra.NewGraph()
	hashMem := map[int]string{}
	reverseHashMem := map[string]int{}
	numNodes := 0
	var dst int
	var src int
	var newNode bool
	for addr := range rssiMap {
		src, newNode, numNodes = rememberHash(addr, hashMem, reverseHashMem, numNodes)
		if newNode {
			graph.AddVertex(src)
		}
		for dstAddr := range rssiMap[addr] {
			dst, newNode, numNodes = rememberHash(dstAddr, hashMem, reverseHashMem, numNodes)
			if newNode {
				graph.AddVertex(dst)
			}
			v := int64(abs(rssiMap[addr][dstAddr]))
			graph.AddArc(src, dst, v)
		}
	}
	src, _, numNodes = rememberHash(forwarderAddr, hashMem, reverseHashMem, numNodes)
	dst, _, numNodes = rememberHash(serverAddr, hashMem, reverseHashMem, numNodes)
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
	ret := []string{}
	for _, ele := range shortest.Path {
		ret = append(ret, hashMem[ele])
	}
	return ret, nil
}
