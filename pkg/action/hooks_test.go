/*
Copyright The Helm Authors.
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

package action

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"helm.sh/helm/v4/pkg/release"
)

func TestConfiguration_hasPostInstallHooks(t *testing.T) {
	type args struct {
		rl *release.Release
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{name: "return true when chart has post-install hooks",
			args: args{rl: &release.Release{
				Hooks: []*release.Hook{{Events: []release.HookEvent{release.HookPostInstall}}},
			},
			},
			want: true,
		},
		{name: "return false when chart does not have post-install hooks",
			args: args{rl: &release.Release{
				Hooks: []*release.Hook{{Events: []release.HookEvent{release.HookPreDelete}}},
			}},
			want: false,
		},
		{name: "return false when chart does not have any hooks",
			args: args{rl: &release.Release{
				Hooks: []*release.Hook{{Events: []release.HookEvent{}}},
			}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Configuration{}
			assert.Equalf(t, tt.want, cfg.hasPostInstallHooks(tt.args.rl), "hasPostInstallHooks(%v)", tt.args.rl)
		})
	}
}
func TestConfiguration_hasPostUpgradeHooks(t *testing.T) {
	type args struct {
		rl *release.Release
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{name: "return true when chart has post-upgrade hooks",
			args: args{rl: &release.Release{
				Hooks: []*release.Hook{{Events: []release.HookEvent{release.HookPostUpgrade}}},
			},
			},
			want: true,
		},
		{name: "return false when chart does not have post-upgrade hooks",
			args: args{rl: &release.Release{
				Hooks: []*release.Hook{{Events: []release.HookEvent{release.HookPreDelete}}},
			}},
			want: false,
		},
		{name: "return false when chart does not have any hooks",
			args: args{rl: &release.Release{
				Hooks: []*release.Hook{{Events: []release.HookEvent{}}},
			}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Configuration{}
			assert.Equalf(t, tt.want, cfg.hasPostUpgradeHooks(tt.args.rl), "hasPostUpgradeHooks(%v)", tt.args.rl)
		})
	}
}
func TestConfiguration_hasPostRollbackHooks(t *testing.T) {
	type args struct {
		rl *release.Release
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{name: "return true when chart has post-rollback hooks",
			args: args{rl: &release.Release{
				Hooks: []*release.Hook{{Events: []release.HookEvent{release.HookPostRollback}}},
			},
			},
			want: true,
		},
		{name: "return false when chart does not have post-rollback hooks",
			args: args{rl: &release.Release{
				Hooks: []*release.Hook{{Events: []release.HookEvent{release.HookPreDelete}}},
			}},
			want: false,
		},
		{name: "return false when chart does not have any hooks",
			args: args{rl: &release.Release{
				Hooks: []*release.Hook{{Events: []release.HookEvent{}}},
			}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Configuration{}
			assert.Equalf(t, tt.want, cfg.hasPostRollbackHooks(tt.args.rl), "hasPostRollbackHooks(%v)", tt.args.rl)
		})
	}
}
