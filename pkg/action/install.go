func (i *Install) failRelease(rel *release.Release, err error) (*release.Release, error) {
	rel.SetStatus(rcommon.StatusFailed, fmt.Sprintf("Release %q failed: %s", i.ReleaseName, err.Error()))
	i.recordRelease(rel) // Ignore the error, since we have another error to deal with.
	if i.RollbackOnFailure {
		i.cfg.Logger().Debug("install failed and rollback-on-failure is set, uninstalling release", "release", i.ReleaseName)
		uninstall := NewUninstall(i.cfg)
		uninstall.DisableHooks = i.DisableHooks
		uninstall.KeepHistory = false
		uninstall.Timeout = i.Timeout
		uninstall.WaitStrategy = i.WaitStrategy
		uninstall.WaitOptions = i.WaitOptions
		if _, uninstallErr := uninstall.Run(i.ReleaseName); uninstallErr != nil {
			return rel, fmt.Errorf("an error occurred while uninstalling the release. original install error: %w: %w", err, uninstallErr)
		}
		return rel, fmt.Errorf("release %s failed, and has been uninstalled due to rollback-on-failure being set: %w", i.ReleaseName, err)
	}
	return rel, err
}