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

package kube

import (
	"bytes"
	"fmt"
	"io"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

type filteredReader struct {
	reader io.ReadCloser

	// a buffer allocation used by the reader
	rawBuffer []byte
	// maxBytes is the max size of the rawBuffer
	maxBytes int

	nread  int
	nready int
	ndone  int

	recovering bool
	eof        bool

	// shouldInclude decides whether to skip an input resource or not
	shouldInclude func(error, *metav1.PartialObjectMetadata) bool
}

var ErrObjectTooLarge = fmt.Errorf("object to decode was longer than maximum allowed size")

const yamlSeparator = "\n---"

func filterManifest(
	reader io.ReadCloser,
	shouldInclude func(error, *metav1.PartialObjectMetadata) bool,
	initialBufferSize int,
	maxBufferSize int,
) io.ReadCloser {
	return &filteredReader{
		reader: reader,

		rawBuffer: make([]byte, initialBufferSize),
		maxBytes:  maxBufferSize,

		nread:  0,
		nready: 0,
		ndone:  0,

		recovering: false,
		eof:        false,

		shouldInclude: shouldInclude,
	}
}

func FilterManifest(reader io.ReadCloser, shouldInclude func(error, *metav1.PartialObjectMetadata) bool) io.ReadCloser {
	return filterManifest(reader, shouldInclude, 1024, 16*1024*1024)
}

// Decode reads the next object from the stream and decodes it.
func (d *filteredReader) Read(buf []byte) (written int, err error) {
	// write as much of the remaining readBuffer as possible
	if d.nready > d.ndone {
		n := copy(buf, d.rawBuffer[d.ndone:d.nready])
		d.ndone += n
		written += n
	}

	// we can't read anything and the buffer is empty
	if d.eof && d.nread == 0 {
		return written, io.EOF
	}

	// we can return if the buffer is filled already
	if len(buf) == written {
		return written, nil
	}

	if d.ndone != d.nready {
		panic("unexpected")
	}

	d.nread = copy(d.rawBuffer, d.rawBuffer[d.nready:d.nread])
	d.ndone, d.nready = 0, 0

	sepLen := len([]byte(yamlSeparator))
	for {
		// go back in buffer to check for '\n---' sequence
		d.nready -= sepLen
		if d.nready < 0 {
			d.nready = 0
		}

		// check if new bytes contain '\n---'
		if i := bytes.Index(d.rawBuffer[d.nready:d.nread], []byte(yamlSeparator)); i >= 0 {
			d.nready = d.nready + i + sepLen
			break
		} else {
			d.nready = d.nread
		}

		if d.nread >= len(d.rawBuffer) {
			if cap(d.rawBuffer) > d.maxBytes {
				d.recovering = true // TODO
				d.nready, d.nread = 0, 0
				return written, ErrObjectTooLarge
			}
			d.rawBuffer = append(d.rawBuffer, 0)
			d.rawBuffer = d.rawBuffer[:cap(d.rawBuffer)]
		}

		if d.eof {
			d.nready = d.nread
			break
		}

		n, err := d.reader.Read(d.rawBuffer[d.nread:])
		d.nread += n
		if err == io.EOF {
			d.eof = true
		} else if err != nil {
			return written, err
		}
	}

	var metadata metav1.PartialObjectMetadata

	test := d.rawBuffer[:d.nready]

	err = yaml.Unmarshal(test, &metadata)
	if !d.shouldInclude(err, &metadata) {
		// skip reading the current [0:d.nready] range
		d.nread = copy(d.rawBuffer, d.rawBuffer[d.nready:d.nread])
		for i := d.nread; i < cap(d.rawBuffer); i++ {
			d.rawBuffer[i] = 0
		}
		d.nready = 0
	}

	n, err := d.Read(buf[written:])
	return written + n, err
}

func (d *filteredReader) Close() error {
	return d.reader.Close()
}
