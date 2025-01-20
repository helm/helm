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
