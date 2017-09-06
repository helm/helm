/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package errors

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrInvalidArgument create a new grpc coded error with codes.InvalidArgument
// indicates user request contains invalid field(missing chart, invalid release name, version, etc)
func ErrInvalidArgument(format string, arg ...interface{}) error {
	return status.Errorf(codes.InvalidArgument, format, arg...)
}

// ErrNotFound create a new grpc coded error with codes.NotFound
func ErrNotFound(format string, arg ...interface{}) error {
	return status.Errorf(codes.NotFound, format, arg...)
}

// ErrConflict create a new grpc coded error with codes.AlreadyExists
func ErrConflict(format string, arg ...interface{}) error {
	return status.Errorf(codes.AlreadyExists, format, arg...)
}

// ErrUnavailable create a new grpc coded error with codes.Unavailable
// client see this kind of error can retry the failed request after some
// backoff periods
func ErrUnavailable(format string, arg ...interface{}) error {
	return status.Errorf(codes.Unavailable, format, arg...)
}

// ErrInternal create a new grpc coded error with codes.Internal
// indicates tiller may encounter some connection issues with k8s apiserver
// or backend metadata storage
func ErrInternal(format string, arg ...interface{}) error {
	return status.Errorf(codes.Internal, format, arg...)
}

// ErrUnknown create a new grpc coded error with codes.Unknown
// indicates tiller may encounter render/marshal/unmarshal issues
func ErrUnknown(format string, arg ...interface{}) error {
	return status.Errorf(codes.Unknown, format, arg...)
}

// IsNotFound is use to check if a error is a grpc coded error with code NotFound
func IsNotFound(err error) bool {
	if e, ok := status.FromError(err); ok {
		return e.Code() == codes.NotFound
	}
	return false
}

// IsInvalidArgument is use to check if a error is a grpc coded error with code
// InvalidArgument
func IsInvalidArgument(err error) bool {
	if e, ok := status.FromError(err); ok {
		return e.Code() == codes.InvalidArgument
	}
	return false
}
