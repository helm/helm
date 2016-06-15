package helmx

import (
	"k8s.io/helm/pkg/helm"
	rls "k8s.io/helm/pkg/proto/hapi/services"
)

// feature toggle helmx APIs while WIP
const EnableNewHelm = false

// These APIs are a temporary abstraction layer that captures the interaction between the current cmd/helm and old
// pkg/helm implementations. Post refactor the cmd/helm package will use the APIs exposed on helm.Client directly.

var Config struct {
	ServAddr string
}

// Soon to be deprecated helm ListReleases API. See pkg/helmx.
func ListReleases(limit int, offset string, sort rls.ListSort_SortBy, order rls.ListSort_SortOrder, filter string) (*rls.ListReleasesResponse, error) {
	if !EnableNewHelm {
		return helm.ListReleases(limit, offset, sort, order, filter)
	}

	opts := []ReleaseListOption{
		ReleaseListLimit(limit),
		ReleaseListOffset(offset),
		ReleaseListFilter(filter),
		ReleaseListSort(int32(sort)),
		ReleaseListOrder(int32(order)),
	}
	return NewClient(HelmHost(Config.ServAddr)).ListReleases(opts...)
}

// Soon to be deprecated helm GetReleaseStatus API. See pkg/helmx.
func GetReleaseStatus(rlsName string) (*rls.GetReleaseStatusResponse, error) {
	if !EnableNewHelm {
		return helm.GetReleaseStatus(rlsName)
	}

	return NewClient(HelmHost(Config.ServAddr)).ReleaseStatus(rlsName)
}

// Soon to be deprecated helm GetReleaseContent API. See pkg/helmx.
func GetReleaseContent(rlsName string) (*rls.GetReleaseContentResponse, error) {
	if !EnableNewHelm {
		return helm.GetReleaseContent(rlsName)
	}

	return NewClient(HelmHost(Config.ServAddr)).ReleaseContent(rlsName)
}

func UpdateRelease(rlsName string) (*rls.UpdateReleaseResponse, error) {
	if !EnableNewHelm {
		return helm.UpdateRelease(rlsName)
	}

	return NewClient(HelmHost(Config.ServAddr)).UpdateRelease(rlsName)
}

// Soon to be deprecated helm InstallRelease API. See pkg/helmx.
func InstallRelease(vals []byte, rlsName, chStr string, dryRun bool) (*rls.InstallReleaseResponse, error) {
	if !EnableNewHelm {
		return helm.InstallRelease(vals, rlsName, chStr, dryRun)
	}

	client := NewClient(HelmHost(Config.ServAddr))
	if dryRun {
		client.Option(DryRun())
	}
	return client.InstallRelease(chStr, ValueOverrides(vals), ReleaseName(rlsName))
}

// Soon to be deprecated helm UninstallRelease API. See pkg/helmx.
func UninstallRelease(rlsName string, dryRun bool) (*rls.UninstallReleaseResponse, error) {
	if !EnableNewHelm {
		return helm.UninstallRelease(rlsName, dryRun)
	}

	client := NewClient(HelmHost(Config.ServAddr))
	if dryRun {
		client.Option(DryRun())
	}
	return client.DeleteRelease(rlsName)
}
