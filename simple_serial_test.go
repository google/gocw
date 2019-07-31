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

package gocw_test

import (
	"bytes"
	"gocw"
	"gocw/mocks"
	"strings"

	"github.com/golang/mock/gomock"
	"testing"
)

func TestNewSimpleSerialFailsOnBadVersion(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	usart := mocks.NewMockUsartInterface(mockCtrl)
	clear := bytes.NewBufferString("xxxxxxxxxxxxxxxxxxx\n")
	gomock.InOrder(
		// flush()
		usart.EXPECT().Write(clear.Bytes()).Return(clear.Len(), nil),
		usart.EXPECT().Flush().Return(nil),
		// checkVersion()
		usart.EXPECT().Flush().Return(nil),
		usart.EXPECT().Write([]byte{'v', '\n'}).Return(2, nil),
		usart.EXPECT().Read(gomock.Any()).
			SetArg(0, []byte{0, 0, 0, 0}).
			Return(4, nil),
	)

	_, err := gocw.NewSimpleSerial(usart)
	if err == nil || !strings.Contains(err.Error(), "Version 1.0 is not supported") {
		t.Errorf("NewSimpleSerial expected to fail with bad version")
	}
}
