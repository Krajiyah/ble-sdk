package util

// Refactored from: https://gist.github.com/hanfang/89d38425699484cd3da80ca086d78129

import (
	"errors"
	mapset "github.com/deckarep/golang-set"
	"github.com/golang-collections/go-datastructures/queue"
)

type queueItem struct {
	cost int
	node string
	path []string
}

func (item queueItem) Compare(other queue.Item) int {
	o := other.(queueItem)
	if item.cost < o.cost {
		return -1
	} else if item.cost == o.cost {
		return 0
	}
	return 1
}

func ShortestPath(graph map[string]map[string]int, src, dst string) ([]string, error) {
	q := queue.NewPriorityQueue(0)
	visited := mapset.NewSet()
	q.Put(queueItem{0, src, []string{}})
	for !q.Empty() {
		entries, err := q.Get(1)
		if err != nil {
			return nil, err
		}
		entry := entries[0].(queueItem)
		cost, node, path := entry.cost, entry.node, entry.path
		if !visited.Contains(node) {
			visited.Add(node)
			newpath := make([]string, len(path))
			copy(newpath, path)
			newpath = append(newpath, node)
			if node == dst {
				return newpath, nil
			}
			for neighbour, c := range graph[node] {
				if !visited.Contains(neighbour) {
					q.Put(queueItem{cost + c, neighbour, newpath})
				}
			}
		}
	}
	return nil, errors.New("could not find path")
}
