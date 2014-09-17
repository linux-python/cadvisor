// Copyright 2014 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Per-container manager.

package manager

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/cadvisor/container"
	"github.com/google/cadvisor/info"
	itest "github.com/google/cadvisor/info/test"
	stest "github.com/google/cadvisor/storage/test"
)

func createManagerAndAddContainers(
	driver *stest.MockStorageDriver,
	containers []string,
	f func(*container.MockContainerHandler),
	t *testing.T,
) *manager {
	if driver == nil {
		driver = &stest.MockStorageDriver{}
	}
	factory := &container.FactoryForMockContainerHandler{
		Name: "factoryForManager",
		PrepareContainerHandlerFunc: func(name string, handler *container.MockContainerHandler) {
			handler.Name = name
			found := false
			for _, c := range containers {
				if c == name {
					found = true
				}
			}
			if !found {
				t.Errorf("Asked to create a container with name %v, which is unknown.", name)
			}
			f(handler)
		},
	}
	container.ClearContainerHandlerFactories()
	container.RegisterContainerHandlerFactory(factory)
	mif, err := New(driver)
	if err != nil {
		t.Fatal(err)
	}
	if ret, ok := mif.(*manager); ok {
		for _, container := range containers {
			ret.containers[container], err = NewContainerData(container, driver)
			if err != nil {
				t.Fatal(err)
			}
		}
		return ret
	}
	t.Fatal("Wrong type")
	return nil
}

// Expect a manager with the specified containers and query. Returns the manager, map of ContainerInfo objects,
// and map of MockContainerHandler objects.}
func expectManagerWithContainers(containers []string, query *info.ContainerInfoRequest, t *testing.T) (*manager, map[string]*info.ContainerInfo, map[string]*container.MockContainerHandler) {
	infosMap := make(map[string]*info.ContainerInfo, len(containers))
	handlerMap := make(map[string]*container.MockContainerHandler, len(containers))

	for _, container := range containers {
		infosMap[container] = itest.GenerateRandomContainerInfo(container, 4, query, 1*time.Second)
	}

	driver := &stest.MockStorageDriver{}
	m := createManagerAndAddContainers(
		driver,
		containers,
		func(h *container.MockContainerHandler) {
			cinfo := infosMap[h.Name]
			stats := cinfo.Stats
			spec := cinfo.Spec
			driver.On(
				"RecentStats",
				h.Name,
				query.NumStats,
			).Return(
				stats,
				nil,
			)

			h.On("ListContainers", container.LIST_SELF).Return(
				[]info.ContainerReference(nil),
				nil,
			)
			h.On("GetSpec").Return(
				spec,
				nil,
			)
			handlerMap[h.Name] = h
		},
		t,
	)

	return m, infosMap, handlerMap
}

func TestGetContainerInfo(t *testing.T) {
	containers := []string{
		"/c1",
		"/c2",
	}

	query := &info.ContainerInfoRequest{
		NumStats: 256,
	}

	m, infosMap, handlerMap := expectManagerWithContainers(containers, query, t)

	returnedInfos := make(map[string]*info.ContainerInfo, len(containers))

	for _, container := range containers {
		cinfo, err := m.GetContainerInfo(container, query)
		if err != nil {
			t.Fatalf("Unable to get info for container %v: %v", container, err)
		}
		returnedInfos[container] = cinfo
	}

	for container, handler := range handlerMap {
		handler.AssertExpectations(t)
		returned := returnedInfos[container]
		expected := infosMap[container]
		if !reflect.DeepEqual(returned, expected) {
			t.Errorf("returned unexpected info for container %v; returned %+v; expected %+v", container, returned, expected)
		}
	}

}

func TestSubcontainersInfo(t *testing.T) {
	containers := []string{
		"/c1",
		"/c2",
	}

	query := &info.ContainerInfoRequest{
		NumStats: 64,
	}

	m, _, _ := expectManagerWithContainers(containers, query, t)

	result, err := m.SubcontainersInfo("/", query)
	if err != nil {
		t.Fatalf("expected to succeed: %s", err)
	}
	if len(result) != len(containers) {
		t.Errorf("expected to received containers: %v, but received: %v", containers, result)
	}
	for _, res := range result {
		found := false
		for _, name := range containers {
			if res.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("unexpected container %q in result, expected one of %v", res.Name, containers)
		}
	}
}
