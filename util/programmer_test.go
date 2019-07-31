// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/gocw/programmer/mocks"
	"github.com/google/gocw/util"

	"github.com/golang/mock/gomock"
)

func TestProgrammerFailsIfEraseFails(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	prog := mocks.NewMockProgrammerInterface(mockCtrl)
	gomock.InOrder(
		// Address block
		prog.EXPECT().Erase().
			Return(fmt.Errorf("erase failed")),
	)

	err := util.ProgramDevice(prog, &util.Segment{0x11223344, []byte{0xaa, 0xbb}})
	if err == nil || !strings.Contains(err.Error(), "erase failed") {
		t.Errorf("ProgramDevice did not fail as expected. Err: %v", err)
	}
}
