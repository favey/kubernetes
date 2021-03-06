/*
Copyright 2014 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package scheduler

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/resources"
	"github.com/golang/glog"
)

// the unused capacity is calculated on a scale of 0-10
// 0 being the lowest priority and 10 being the highest
func calculateScore(requested, capacity int, node string) int {
	if capacity == 0 {
		return 0
	}
	if requested > capacity {
		glog.Errorf("Combined requested resources from existing pods exceeds capacity on minion: %s", node)
		return 0
	}
	return ((capacity - requested) * 10) / capacity
}

// Calculate the occupancy on a node.  'node' has information about the resources on the node.
// 'pods' is a list of pods currently scheduled on the node.
func calculateOccupancy(pod api.Pod, node api.Node, pods []api.Pod) HostPriority {
	totalCPU := 0
	totalMemory := 0
	for _, existingPod := range pods {
		for _, container := range existingPod.Spec.Containers {
			totalCPU += container.CPU
			totalMemory += container.Memory
		}
	}
	// Add the resources requested by the current pod being scheduled.
	// This also helps differentiate between differently sized, but empty, minions.
	for _, container := range pod.Spec.Containers {
		totalCPU += container.CPU
		totalMemory += container.Memory
	}

	cpuScore := calculateScore(totalCPU, resources.GetIntegerResource(node.Spec.Capacity, resources.CPU, 0), node.Name)
	memoryScore := calculateScore(totalMemory, resources.GetIntegerResource(node.Spec.Capacity, resources.Memory, 0), node.Name)
	glog.V(4).Infof("Least Requested Priority, AbsoluteRequested: (%d, %d) Score:(%d, %d)", totalCPU, totalMemory, cpuScore, memoryScore)

	return HostPriority{
		host:  node.Name,
		score: int((cpuScore + memoryScore) / 2),
	}
}

// LeastRequestedPriority is a priority function that favors nodes with fewer requested resources.
// It calculates the percentage of memory and CPU requested by pods scheduled on the node, and prioritizes
// based on the minimum of the average of the fraction of requested to capacity.
// Details: (Sum(requested cpu) / Capacity + Sum(requested memory) / Capacity) * 50
func LeastRequestedPriority(pod api.Pod, podLister PodLister, minionLister MinionLister) (HostPriorityList, error) {
	nodes, err := minionLister.List()
	if err != nil {
		return HostPriorityList{}, err
	}
	podsToMachines, err := MapPodsToMachines(podLister)

	list := HostPriorityList{}
	for _, node := range nodes.Items {
		list = append(list, calculateOccupancy(pod, node, podsToMachines[node.Name]))
	}
	return list, nil
}
