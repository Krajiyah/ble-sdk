package util

import (
	"hash/fnv"

	"github.com/RyanCarrier/dijkstra"
)

func abs(x int) int {
	if x >= 0 {
		return x
	}
	return x * -1
}

func hash(s string) int {
	h := fnv.New32a()
	h.Write([]byte(s))
	return int(h.Sum32())
}

func rememberHash(s string, mem map[int]string) int {
	ret := hash(s)
	mem[ret] = s
	return ret
}

// ShortestPathToServer tells forwarder which path to take to forward packets to server
func ShortestPathToServer(forwarderAddr string, serverAddr string, rssiMap map[string]map[string]int) ([]string, error) {
	graph := dijkstra.NewGraph()
	hashMem := map[int]string{}
	for addr := range rssiMap {
		src := rememberHash(addr, hashMem)
		graph.AddVertex(src)
		for dstAddr := range rssiMap[addr] {
			dst := rememberHash(dstAddr, hashMem)
			graph.AddArc(src, dst, int64(abs(rssiMap[addr][dstAddr])))
		}
	}
	src := rememberHash(forwarderAddr, hashMem)
	dst := rememberHash(serverAddr, hashMem)
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
