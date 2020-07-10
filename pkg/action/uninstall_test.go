package action

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUninstall_deleteRelease(t *testing.T) {
	is := assert.New(t)
	rel := releaseStub()
	rel.Chart =buildChart(withKeepAnnoManifestTemplate())
	rel.Manifest = manifestWithKeepAnno
	config := actionConfigFixture(t)
	unisAction := NewUninstall(config)
	str, errs := unisAction.deleteRelease(rel)
	is.Len(errs,0)
	is.Equal("pod/pod-keep\n",str)
}
