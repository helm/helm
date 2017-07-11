// Code generated by protoc-gen-go.
// source: hapi/chart/metadata.proto
// DO NOT EDIT!

package chart

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

type Metadata_Engine int32

const (
	Metadata_UNKNOWN Metadata_Engine = 0
	Metadata_GOTPL   Metadata_Engine = 1
)

var Metadata_Engine_name = map[int32]string{
	0: "UNKNOWN",
	1: "GOTPL",
}
var Metadata_Engine_value = map[string]int32{
	"UNKNOWN": 0,
	"GOTPL":   1,
}

func (x Metadata_Engine) String() string {
	return proto.EnumName(Metadata_Engine_name, int32(x))
}
func (Metadata_Engine) EnumDescriptor() ([]byte, []int) { return fileDescriptor2, []int{1, 0} }

// Maintainer describes a Chart maintainer.
type Maintainer struct {
	// Name is a user name or organization name
	Name string `protobuf:"bytes,1,opt,name=name" json:"name,omitempty"`
	// Email is an optional email address to contact the named maintainer
	Email string `protobuf:"bytes,2,opt,name=email" json:"email,omitempty"`
}

func (m *Maintainer) Reset()                    { *m = Maintainer{} }
func (m *Maintainer) String() string            { return proto.CompactTextString(m) }
func (*Maintainer) ProtoMessage()               {}
func (*Maintainer) Descriptor() ([]byte, []int) { return fileDescriptor2, []int{0} }

func (m *Maintainer) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *Maintainer) GetEmail() string {
	if m != nil {
		return m.Email
	}
	return ""
}

// 	Metadata for a Chart file. This models the structure of a Chart.yaml file.
//
// 	Spec: https://k8s.io/helm/blob/master/docs/design/chart_format.md#the-chart-file
type Metadata struct {
	// The name of the chart
	Name string `protobuf:"bytes,1,opt,name=name" json:"name,omitempty"`
	// The URL to a relevant project page, git repo, or contact person
	Home string `protobuf:"bytes,2,opt,name=home" json:"home,omitempty"`
	// Source is the URL to the source code of this chart
	Sources []string `protobuf:"bytes,3,rep,name=sources" json:"sources,omitempty"`
	// A SemVer 2 conformant version string of the chart
	Version string `protobuf:"bytes,4,opt,name=version" json:"version,omitempty"`
	// A one-sentence description of the chart
	Description string `protobuf:"bytes,5,opt,name=description" json:"description,omitempty"`
	// A list of string keywords
	Keywords []string `protobuf:"bytes,6,rep,name=keywords" json:"keywords,omitempty"`
	// A list of name and URL/email address combinations for the maintainer(s)
	Maintainers []*Maintainer `protobuf:"bytes,7,rep,name=maintainers" json:"maintainers,omitempty"`
	// The name of the template engine to use. Defaults to 'gotpl'.
	Engine string `protobuf:"bytes,8,opt,name=engine" json:"engine,omitempty"`
	// The URL to an icon file.
	Icon string `protobuf:"bytes,9,opt,name=icon" json:"icon,omitempty"`
	// The API Version of this chart.
	ApiVersion string `protobuf:"bytes,10,opt,name=apiVersion" json:"apiVersion,omitempty"`
	// The condition to check to enable chart
	Condition string `protobuf:"bytes,11,opt,name=condition" json:"condition,omitempty"`
	// The tags to check to enable chart
	Tags string `protobuf:"bytes,12,opt,name=tags" json:"tags,omitempty"`
	// The version of the application enclosed inside of this chart.
	AppVersion string `protobuf:"bytes,13,opt,name=appVersion" json:"appVersion,omitempty"`
	// Whether or not this chart is deprecated
	Deprecated bool `protobuf:"varint,14,opt,name=deprecated" json:"deprecated,omitempty"`
	// TillerVersion is a SemVer constraints on what version of Tiller is required.
	// See SemVer ranges here: https://github.com/Masterminds/semver#basic-comparisons
	TillerVersion string `protobuf:"bytes,15,opt,name=tillerVersion" json:"tillerVersion,omitempty"`
	// Options is an unstructured key value map for user to store arbitrary data.
	Options map[string]string `protobuf:"bytes,16,rep,name=options" json:"options,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
}

func (m *Metadata) Reset()                    { *m = Metadata{} }
func (m *Metadata) String() string            { return proto.CompactTextString(m) }
func (*Metadata) ProtoMessage()               {}
func (*Metadata) Descriptor() ([]byte, []int) { return fileDescriptor2, []int{1} }

func (m *Metadata) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *Metadata) GetHome() string {
	if m != nil {
		return m.Home
	}
	return ""
}

func (m *Metadata) GetSources() []string {
	if m != nil {
		return m.Sources
	}
	return nil
}

func (m *Metadata) GetVersion() string {
	if m != nil {
		return m.Version
	}
	return ""
}

func (m *Metadata) GetDescription() string {
	if m != nil {
		return m.Description
	}
	return ""
}

func (m *Metadata) GetKeywords() []string {
	if m != nil {
		return m.Keywords
	}
	return nil
}

func (m *Metadata) GetMaintainers() []*Maintainer {
	if m != nil {
		return m.Maintainers
	}
	return nil
}

func (m *Metadata) GetEngine() string {
	if m != nil {
		return m.Engine
	}
	return ""
}

func (m *Metadata) GetIcon() string {
	if m != nil {
		return m.Icon
	}
	return ""
}

func (m *Metadata) GetApiVersion() string {
	if m != nil {
		return m.ApiVersion
	}
	return ""
}

func (m *Metadata) GetCondition() string {
	if m != nil {
		return m.Condition
	}
	return ""
}

func (m *Metadata) GetTags() string {
	if m != nil {
		return m.Tags
	}
	return ""
}

func (m *Metadata) GetAppVersion() string {
	if m != nil {
		return m.AppVersion
	}
	return ""
}

func (m *Metadata) GetDeprecated() bool {
	if m != nil {
		return m.Deprecated
	}
	return false
}

func (m *Metadata) GetTillerVersion() string {
	if m != nil {
		return m.TillerVersion
	}
	return ""
}

func (m *Metadata) GetOptions() map[string]string {
	if m != nil {
		return m.Options
	}
	return nil
}

func init() {
	proto.RegisterType((*Maintainer)(nil), "hapi.chart.Maintainer")
	proto.RegisterType((*Metadata)(nil), "hapi.chart.Metadata")
	proto.RegisterEnum("hapi.chart.Metadata_Engine", Metadata_Engine_name, Metadata_Engine_value)
}

func init() { proto.RegisterFile("hapi/chart/metadata.proto", fileDescriptor2) }

var fileDescriptor2 = []byte{
	// 409 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x6c, 0x52, 0x5d, 0x6b, 0xd4, 0x40,
	0x14, 0x35, 0xdd, 0x8f, 0x6c, 0x6e, 0x5a, 0x5d, 0x2e, 0x52, 0xc6, 0x22, 0x12, 0x17, 0x1f, 0xf6,
	0x69, 0x0b, 0x0a, 0x52, 0xea, 0x9b, 0x50, 0x7c, 0xd0, 0xee, 0x4a, 0xf0, 0x03, 0x7c, 0x1b, 0x93,
	0x4b, 0x77, 0xe8, 0x66, 0x26, 0xcc, 0x4c, 0x2b, 0xfb, 0x63, 0xfd, 0x2f, 0x32, 0x37, 0x99, 0x6e,
	0x0a, 0xbe, 0xdd, 0x73, 0xce, 0xdc, 0x93, 0x9c, 0xcb, 0x81, 0x17, 0x5b, 0xd9, 0xaa, 0xf3, 0x6a,
	0x2b, 0xad, 0x3f, 0x6f, 0xc8, 0xcb, 0x5a, 0x7a, 0xb9, 0x6a, 0xad, 0xf1, 0x06, 0x21, 0x48, 0x2b,
	0x96, 0x16, 0xef, 0x01, 0xae, 0xa5, 0xd2, 0x5e, 0x2a, 0x4d, 0x16, 0x11, 0xc6, 0x5a, 0x36, 0x24,
	0x92, 0x22, 0x59, 0x66, 0x25, 0xcf, 0xf8, 0x1c, 0x26, 0xd4, 0x48, 0xb5, 0x13, 0x47, 0x4c, 0x76,
	0x60, 0xf1, 0x77, 0x0c, 0xb3, 0xeb, 0xde, 0xf6, 0xbf, 0x6b, 0x08, 0xe3, 0xad, 0x69, 0xa8, 0xdf,
	0xe2, 0x19, 0x05, 0xa4, 0xce, 0xdc, 0xd9, 0x8a, 0x9c, 0x18, 0x15, 0xa3, 0x65, 0x56, 0x46, 0x18,
	0x94, 0x7b, 0xb2, 0x4e, 0x19, 0x2d, 0xc6, 0xbc, 0x10, 0x21, 0x16, 0x90, 0xd7, 0xe4, 0x2a, 0xab,
	0x5a, 0x1f, 0xd4, 0x09, 0xab, 0x43, 0x0a, 0xcf, 0x60, 0x76, 0x4b, 0xfb, 0x3f, 0xc6, 0xd6, 0x4e,
	0x4c, 0xd9, 0xf6, 0x01, 0xe3, 0x05, 0xe4, 0xcd, 0x43, 0x3c, 0x27, 0xd2, 0x62, 0xb4, 0xcc, 0xdf,
	0x9e, 0xae, 0x0e, 0x07, 0x58, 0x1d, 0xd2, 0x97, 0xc3, 0xa7, 0x78, 0x0a, 0x53, 0xd2, 0x37, 0x4a,
	0x93, 0x98, 0xf1, 0x27, 0x7b, 0x14, 0x72, 0xa9, 0xca, 0x68, 0x91, 0x75, 0xb9, 0xc2, 0x8c, 0xaf,
	0x00, 0x64, 0xab, 0x7e, 0xf4, 0x01, 0x80, 0x95, 0x01, 0x83, 0x2f, 0x21, 0xab, 0x8c, 0xae, 0x15,
	0x27, 0xc8, 0x59, 0x3e, 0x10, 0xc1, 0xd1, 0xcb, 0x1b, 0x27, 0x8e, 0x3b, 0xc7, 0x30, 0x77, 0x8e,
	0x6d, 0x74, 0x3c, 0x89, 0x8e, 0x91, 0x09, 0x7a, 0x4d, 0xad, 0xa5, 0x4a, 0x7a, 0xaa, 0xc5, 0xd3,
	0x22, 0x59, 0xce, 0xca, 0x01, 0x83, 0x6f, 0xe0, 0xc4, 0xab, 0xdd, 0x8e, 0x6c, 0xb4, 0x78, 0xc6,
	0x16, 0x8f, 0x49, 0xfc, 0x00, 0xa9, 0xe1, 0x1b, 0x3a, 0x31, 0xe7, 0xcb, 0xbc, 0x7e, 0x74, 0x99,
	0xd8, 0x9a, 0x4d, 0xf7, 0xe6, 0x4a, 0x7b, 0xbb, 0x2f, 0xe3, 0xc6, 0xd9, 0x25, 0x1c, 0x0f, 0x05,
	0x9c, 0xc3, 0xe8, 0x96, 0xf6, 0x7d, 0x07, 0xc2, 0x18, 0x9a, 0x73, 0x2f, 0x77, 0x77, 0xb1, 0x03,
	0x1d, 0xb8, 0x3c, 0xba, 0x48, 0x16, 0x05, 0x4c, 0xaf, 0xba, 0x73, 0xe6, 0x90, 0x7e, 0x5f, 0x7f,
	0x5e, 0x6f, 0x7e, 0xae, 0xe7, 0x4f, 0x30, 0x83, 0xc9, 0xa7, 0xcd, 0xb7, 0xaf, 0x5f, 0xe6, 0xc9,
	0xc7, 0xf4, 0xd7, 0x84, 0xff, 0xe2, 0xf7, 0x94, 0x3b, 0xfb, 0xee, 0x5f, 0x00, 0x00, 0x00, 0xff,
	0xff, 0xc8, 0x2c, 0xa3, 0x3c, 0xd0, 0x02, 0x00, 0x00,
}
