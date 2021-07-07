/*
	Copyright 2019 NetFoundry, Inc.

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

	https://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
*/

package zsshlib

import (
	"github.com/natefinch/npipe"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"sync"
	"time"
)

var warnOnce = sync.Once{}
var pipePresent = true

func sshAuthMethodAgent() ssh.AuthMethod {
	if !pipePresent {
		return nil
	}

	if sshAgent, err := npipe.DialTimeout(`\\.\pipe\openssh-zssh-agent`, 1*time.Second); err == nil {
		return ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers)
	} else {
		warnOnce.Do(func() {
			pipePresent = false
			logrus.WithError(err).Debug("could not connect to openssh zssh-agent pipe, will not be tried again this run")
		})
	}
	return nil
}
