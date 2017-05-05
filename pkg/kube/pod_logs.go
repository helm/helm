package kube

import (
	"time"
	"k8s.io/kubernetes/pkg/kubectl/cmd"
	"io"
	"math"
	"errors"
	"k8s.io/kubernetes/pkg/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/apimachinery/pkg/runtime"
)

// Mimics kubectl logs functionality

var (
	selectorTail int64 = 10
)

// All options that can be passed to kubectl logs
type LogOptions struct {
	// Specify if the logs should be streamed.
	Follow bool
	// Include timestamps on each line in the log output
	Timestamps bool
	// Maximum bytes of logs to return. Defaults to no limit.
	LimitBytes int64
	// If true, print the logs for the previous instance of the container in a pod if it exists.
	Previous bool
	// Lines of recent log file to display. Defaults to -1 with no selector, showing all log lines otherwise 10, if a selector is provided.
	Tail int64
	// Only return logs after a specific date (RFC3339). Defaults to all logs. Only one of since-time / since may be used.
	SinceTime time.Time
	// Only return logs newer than a relative duration like 5s, 2m, or 3h. Defaults to all logs. Only one of since-time / since may be used.
	Since time.Duration
	// Print the logs of this container
	Container string
	// Selector (label query) to filter on.
	Selector string
	// Namespace to query for logs
	Namespace string
	// Resource to query for logs
	Resource string
}

func NewOptions() *LogOptions {
	return &LogOptions{
		Follow: false,
		Timestamps: false,
		LimitBytes: 0,
		Previous: false,
		Tail: -1,
		SinceTime: nil,
		Since: nil,
		Container: "",
		Selector: "",
		Namespace: "",
		Resource: "",
	}
}

func (o *LogOptions) ExecuteLogRequest(out io.Writer) {
	f := cmdutil.NewFactory(nil)
	Complete(o, f, out)
}

func Complete(opts *LogOptions, f cmdutil.Factory, out io.Writer) (*cmd.LogsOptions, error) {
	o := &cmd.LogsOptions{}
	containerName := opts.Container
	selector := opts.Selector
	if len(opts.Resource) != 0 && len(opts.Selector) != 0 {
		return nil, errors.New("Specify either a selector or a resource, not both")
	}
	o.Namespace = opts.Namespace
	if o.Namespace == "" {
		return nil, errors.New("Namespace is required")
	}

	logOptions := &api.PodLogOptions{
		Container:  containerName,
		Follow:     opts.Follow,
		Previous:   opts.Previous,
		Timestamps: opts.Timestamps,
	}
	if opts.SinceTime {
		t := metav1.NewTime(opts.SinceTime)
		logOptions.SinceTime = &t
	}
	if opts.LimitBytes != 0 {
		logOptions.LimitBytes = &opts.LimitBytes
	}
	if opts.Tail != -1 {
		logOptions.TailLines = &opts.Tail
	}
	if opts.Since {
		// round up to the nearest second
		sec := int64(math.Ceil(opts.Since.Seconds()))
		logOptions.SinceSeconds = &sec
	}
	o.Options = logOptions
	o.LogsForObject = f.LogsForObject
	o.ClientMapper = resource.ClientMapperFunc(f.ClientForMapping)
	o.Out = out

	if len(selector) != 0 {
		if logOptions.Follow {
			return nil, errors.New("only one of follow (-f) or selector (-l) is allowed")
		}
		if len(logOptions.Container) != 0 {
			return nil, errors.New( "a container cannot be specified when using a selector (-l)")
		}
		if logOptions.TailLines == nil {
			logOptions.TailLines = &selectorTail
		}
	}

	mapper, typer := f.Object()
	decoder := f.Decoder(true)
	if o.Object == nil {
		builder := resource.NewBuilder(mapper, typer, o.ClientMapper, decoder).
			NamespaceParam(o.Namespace).DefaultNamespace().
			SingleResourceType()
		if o.ResourceArg != "" {
			builder.ResourceNames("pods", o.ResourceArg)
		}
		if selector != "" {
			builder.ResourceTypes("pods").SelectorParam(selector)
		}
		infos, err := builder.Do().Infos()
		if err != nil {
			return nil, err
		}
		if selector == "" && len(infos) != 1 {
			return nil, errors.New("expected a resource")
		}
		o.Object = infos[0].Object
	}

	return o, nil
}

func Validate(o *cmd.LogsOptions) error {
	logsOptions, ok := o.Options.(*api.PodLogOptions)
	if !ok {
		return errors.New("unexpected logs options object")
	}
	if errs := validation.ValidatePodLogOptions(logsOptions); len(errs) > 0 {
		return errs.ToAggregate()
	}

	return nil
}

// RunLogs retrieves a pod log
func RunLogs(o *cmd.LogsOptions) error {
	switch t := o.Object.(type) {
	case *api.PodList:
		for _, p := range t.Items {
			if err := getLogs(o, &p); err != nil {
				return err
			}
		}
		return nil
	default:
		return getLogs(o, o.Object)
	}
}

func getLogs(o *cmd.LogsOptions, obj runtime.Object) error {
	req, err := o.LogsForObject(obj, o.Options)
	if err != nil {
		return err
	}

	readCloser, err := req.Stream()
	if err != nil {
		return err
	}
	defer readCloser.Close()

	_, err = io.Copy(o.Out, readCloser)
	return err
}