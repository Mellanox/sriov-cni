// Copyright 2025 sriov-cni authors
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
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containernetworking/plugins/pkg/testutils"
)

var _ = Describe("PCIAllocator", func() {
	var targetNetNS ns.NetNS
	var err error

	AfterEach(func() {
		if targetNetNS != nil {
			targetNetNS.Close()
			err = testutils.UnmountNS(targetNetNS)
		}
	})

	Context("IsAllocated", func() {
		It("Assuming is not allocated", func() {
			allocator := NewPCIAllocator(ts.dirRoot)
			isAllocated, err := allocator.IsAllocated("0000:af:00.1")
			Expect(err).ToNot(HaveOccurred())
			Expect(isAllocated).To(BeFalse())
		})

		It("Assuming is allocated and namespace exist", func() {
			targetNetNS, err = testutils.NewNS()
			Expect(err).NotTo(HaveOccurred())
			allocator := NewPCIAllocator(ts.dirRoot)

			err = allocator.SaveAllocatedPCI("0000:af:00.1", targetNetNS.Path())
			Expect(err).ToNot(HaveOccurred())

			isAllocated, err := allocator.IsAllocated("0000:af:00.1")
			Expect(err).ToNot(HaveOccurred())
			Expect(isAllocated).To(BeTrue())
		})

		It("Assuming is allocated and namespace doesn't exist", func() {
			targetNetNS, err = testutils.NewNS()
			Expect(err).NotTo(HaveOccurred())

			allocator := NewPCIAllocator(ts.dirRoot)
			err = allocator.SaveAllocatedPCI("0000:af:00.1", targetNetNS.Path())
			Expect(err).ToNot(HaveOccurred())
			err = targetNetNS.Close()
			Expect(err).ToNot(HaveOccurred())
			err = testutils.UnmountNS(targetNetNS)
			Expect(err).ToNot(HaveOccurred())

			isAllocated, err := allocator.IsAllocated("0000:af:00.1")
			Expect(err).ToNot(HaveOccurred())
			Expect(isAllocated).To(BeFalse())
		})
	})
})
