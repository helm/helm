package storage

import rspb "k8s.io/helm/pkg/proto/hapi/release"

// FilterFunc returns true if the release object satisfies
// the predicate of the underlying func.
type FilterFunc func(*rspb.Release) bool

// Check applies the FilterFunc to the release object.
func (fn FilterFunc) Check(rls *rspb.Release) bool {
	if rls == nil {
		return false
	}
	return fn(rls) 
}

// Any returns a FilterFunc that filters a list of releases
// determined by the predicate 'f0 || f1 || ... || fn'.
func Any(filters ...FilterFunc) FilterFunc {
	return func(rls *rspb.Release) bool {
		for _, filter := range filters {
			if filter(rls) {
				return true
			}
		}
		return false
	}
}

// All returns a FilterFunc that filters a list of releases
// determined by the predicate 'f0 && f1 && ... && fn'.
func All(filters ...FilterFunc) FilterFunc {
	return func(rls *rspb.Release) bool {
		for _, filter := range filters {
			if !filter(rls) {
				return false
			}
		}
		return true
	}
}

// StatusFilter filters a set of releases by status code.
func StatusFilter(status rspb.Status_Code) FilterFunc {
	return FilterFunc(func(rls *rspb.Release) bool {
		if rls == nil {
			return true
		}
		return rls.GetInfo().GetStatus().Code == status
	})
}