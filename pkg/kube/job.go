package kube

import (
	"bytes"
	_ "fmt"
	"io"
	"sort"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/resource"
)

type WatchFeed interface {
	WriteJobLogChunk(*JobLogChunk) error
}

type LogLine struct {
	Timestamp string
	Data      string
}

type JobLogChunk struct {
	JobName       string
	PodName       string
	ContainerName string
	LogLines      []LogLine
}

type WriteJobLogChunkFunc func(*JobLogChunk) error

func (f WriteJobLogChunkFunc) WriteJobLogChunk(chunk *JobLogChunk) error {
	return f(chunk)
}

type WatchMonitor struct {
	kube      *Client
	timeout   time.Duration
	watchFeed WatchFeed

	Namespace    string
	ResourceName string
	UID          types.UID
}

type PodWatchMonitor struct {
	WatchMonitor

	Manifest *core.Pod

	InitContainersNames          []string
	ProcessedInitContainersNames []string
	ProcessedInitContainersIDs   []string

	ContainersNames          []string
	ProcessedContainersNames []string
	ProcessedContainersIDs   []string
}

func (pod *PodWatchMonitor) GetMonitoredContainersNames() []string {
	res := make([]string, 0)

FilterProcessedContainersNames:
	for _, name := range pod.ContainersNames {
		for _, processedContainerName := range pod.ProcessedContainersNames {
			if processedContainerName == name {
				continue FilterProcessedContainersNames
			}
		}
		res = append(res, name)
	}

	return res
}

func (pod *PodWatchMonitor) SetContainerProcessed(containerName string, containerID string) {
	pod.ProcessedContainersNames = append(pod.ProcessedContainersNames, containerName)
	pod.ProcessedContainersIDs = append(pod.ProcessedContainersIDs, containerID)
}

func (pod *PodWatchMonitor) GetMonitoredInitContainersNames() []string {
	res := make([]string, 0)

FilterProcessedInitContainersNames:
	for _, name := range pod.InitContainersNames {
		for _, processedInitContainerName := range pod.ProcessedInitContainersNames {
			if processedInitContainerName == name {
				continue FilterProcessedInitContainersNames
			}
		}
		res = append(res, name)
	}

	return res
}

func (pod *PodWatchMonitor) SetInitContainerProcessed(containerName string, containerID string) {
	pod.ProcessedInitContainersNames = append(pod.ProcessedInitContainersNames, containerName)
	pod.ProcessedInitContainersIDs = append(pod.ProcessedInitContainersIDs, containerID)
}

func (pod *PodWatchMonitor) RefreshManifest() error {
	client, err := pod.kube.ClientSet()
	if err != nil {
		return err
	}

	manifest, err := client.Core().
		Pods(pod.Namespace).
		Get(pod.ResourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	pod.Manifest = manifest

	return nil
}

func (pod *PodWatchMonitor) GetReadyCondition() (res *core.PodCondition) {
	for i, _ := range pod.Manifest.Status.Conditions {
		if pod.Manifest.Status.Conditions[i].Type == "Ready" {
			res = &pod.Manifest.Status.Conditions[i]
			break
		}
	}
	return
}

func (pod *PodWatchMonitor) GetInitContainerStatus(containerName string) (res *core.ContainerStatus) {
	for i, _ := range pod.Manifest.Status.InitContainerStatuses {
		if pod.Manifest.Status.InitContainerStatuses[i].Name == containerName {
			res = &pod.Manifest.Status.InitContainerStatuses[i]
			break
		}
	}
	return
}

func (pod *PodWatchMonitor) GetContainerStatus(containerName string) (res *core.ContainerStatus) {
	for i, _ := range pod.Manifest.Status.ContainerStatuses {
		if pod.Manifest.Status.ContainerStatuses[i].Name == containerName {
			res = &pod.Manifest.Status.ContainerStatuses[i]
			break
		}
	}
	return
}

func (pod *PodWatchMonitor) GetContainerLogs(containerName string) ([]LogLine, error) {
	client, err := pod.kube.ClientSet()
	if err != nil {
		return nil, err
	}

	req := client.Core().
		Pods(pod.Namespace).
		GetLogs(pod.ResourceName, &core.PodLogOptions{
			Container:  containerName,
			Timestamps: true,
		})

	readCloser, err := req.Stream()
	if err != nil {
		return nil, err
	}
	defer readCloser.Close()

	buf := bytes.Buffer{}
	_, err = io.Copy(&buf, readCloser)

	res := make([]LogLine, 0)
	for _, line := range strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n") {
		lineParts := strings.SplitN(line, " ", 2)
		// TODO: receive only new log lines, save state into PodWatchMonitor
		if len(lineParts) == 2 {
			ll := LogLine{
				Timestamp: lineParts[0],
				Data:      lineParts[1],
			}
			res = append(res, ll)
		}
	}

	return res, nil
}

type JobWatchMonitor struct {
	WatchMonitor

	MonitoredPodsQueue []*PodWatchMonitor
	ProcessedPodsUIDs  []types.UID
}

func (job *JobWatchMonitor) WaitTillResourceVersionAdded(resourceVersion string, jobInfo *resource.Info) error {
	w, err := resource.
		NewHelper(jobInfo.Client, jobInfo.Mapping).
		WatchSingle(job.Namespace, job.ResourceName, resourceVersion)
	if err != nil {
		return err
	}

	_, err = watch.Until(job.timeout, w, func(e watch.Event) (bool, error) {
		if e.Type == watch.Added {
			return true, nil
		}
		return false, nil
	})

	return err
}

func (job *JobWatchMonitor) TakeNextMonitoredPod() *PodWatchMonitor {
	if len(job.MonitoredPodsQueue) == 0 {
		return nil
	}

	var res *PodWatchMonitor
	res, job.MonitoredPodsQueue = job.MonitoredPodsQueue[0], job.MonitoredPodsQueue[1:]
	return res
}

func (job *JobWatchMonitor) SetPodProcessed(uid types.UID) {
	job.ProcessedPodsUIDs = append(job.ProcessedPodsUIDs, uid)
}

func (job *JobWatchMonitor) SchedulePodMonitoring(pod *PodWatchMonitor) {
	job.MonitoredPodsQueue = append(job.MonitoredPodsQueue, pod)
}

func (job *JobWatchMonitor) RefreshMonitoredPods() error {
	job.kube.Log("RefreshMonitoredPods") // TODO: remove

	client, err := job.kube.ClientSet()
	if err != nil {
		return err
	}

	jobManifest, err := client.Batch().
		Jobs(job.Namespace).
		Get(job.ResourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	job.kube.Log("jobManifest: %+v", jobManifest) // TODO: remove

	selector, err := metav1.LabelSelectorAsSelector(jobManifest.Spec.Selector)
	if err != nil {
		return err
	}

	podList, err := client.Core().
		Pods(job.Namespace).
		List(metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return err
	}
	job.kube.Log("podList: %+v", podList) // TODO: remove

	// TODO filter out pods that does not belong to controller-uid=job-uid

	// Add new pods to monitor queue in chronological order by creation timestamp
	podItems := make([]core.Pod, 0)
	for _, item := range podList.Items {
		podItems = append(podItems, item)
	}
	sort.Slice(podItems, func(i, j int) bool {
		return podItems[i].CreationTimestamp.Time.Before(podItems[j].CreationTimestamp.Time)
	})

searchNewPods:
	for _, item := range podItems {
		// filter out under processing
		for _, monitoredPod := range job.MonitoredPodsQueue {
			// TODO is there a need to check resource-version change?
			if monitoredPod.UID == item.UID {
				continue searchNewPods
			}
		}
		// filter out already processed
		for _, processedPodUID := range job.ProcessedPodsUIDs {
			if processedPodUID == item.UID {
				continue searchNewPods
			}
		}

		pod := &PodWatchMonitor{
			WatchMonitor: WatchMonitor{
				kube:      job.kube,
				timeout:   job.timeout,
				watchFeed: job.watchFeed,

				Namespace:    job.Namespace,
				ResourceName: item.Name,
				UID:          item.UID,
			},
		}
		if err = pod.RefreshManifest(); err != nil {
			return err
		}
		for _, containerConf := range pod.Manifest.Spec.InitContainers {
			pod.InitContainersNames = append(pod.InitContainersNames, containerConf.Name)
		}
		for _, containerConf := range pod.Manifest.Spec.Containers {
			pod.ContainersNames = append(pod.ContainersNames, containerConf.Name)
		}

		job.MonitoredPodsQueue = append(job.MonitoredPodsQueue, pod)
	}

	job.kube.Log("RefreshMonitoredPods done: MonitoredPodsQueue: %+v", job.MonitoredPodsQueue) // TODO: remove

	return nil
}

func (c *Client) watchJobTillDone(jobInfo *resource.Info, watchFeed WatchFeed, timeout time.Duration) error {
	if jobInfo.Mapping.GroupVersionKind.Kind != "Job" {
		return nil
	}

	uid, err := jobInfo.Mapping.UID(jobInfo.Object)
	if err != nil {
		return err
	}

	job := &JobWatchMonitor{
		WatchMonitor: WatchMonitor{
			kube:      c,
			timeout:   timeout,
			watchFeed: watchFeed,

			Namespace:    jobInfo.Namespace,
			ResourceName: jobInfo.Name,
			UID:          uid,
		},
	}

	if err := job.WaitTillResourceVersionAdded(jobInfo.ResourceVersion, jobInfo); err != nil {
		return err // TODO
	}

	if err = job.RefreshMonitoredPods(); err != nil {
		return err // TODO
	}

	var processPod *PodWatchMonitor

	// TODO: split method into corresponding functions

TakeNextMonitoredPod:
	for {
		if processPod = job.TakeNextMonitoredPod(); processPod == nil {
			break
		}

		if err := processPod.RefreshManifest(); err != nil {
			c.Log("Pod %s refresh manifest failed: %s", processPod.ResourceName, err)
			// TODO stream system-error to feed
			job.SetPodProcessed(processPod.UID)
			continue TakeNextMonitoredPod
		}

		c.Log("Pod manifest refreshed, ResourceVersion: %s", processPod.Manifest.ResourceVersion)

		if processPod.Manifest.Status.Phase == core.PodPending {
			c.Log("Pod %s is in PENDING state", processPod.ResourceName)

			if podReadyCondition := processPod.GetReadyCondition(); podReadyCondition != nil {
				c.Log("Pod %s ready condition: %+v", processPod.ResourceName, podReadyCondition)
				if podReadyCondition.Status != core.ConditionTrue {
					// TODO: init-containers-statuses
					for _, containerStatus := range processPod.Manifest.Status.ContainerStatuses {
						if containerStatus.Ready {
							continue
						}
						if containerStatus.State.Waiting != nil {
							c.Log("Pod %s container %s is in waiting state: %s: %s", processPod.ResourceName, containerStatus.Name, containerStatus.State.Waiting.Reason, containerStatus.State.Waiting.Message)

							switch containerStatus.State.Waiting.Reason {
							case "ImagePullBackOff", "ErrImagePull":
								// TODO stream bad_image user-error
								processPod.SetContainerProcessed(containerStatus.Name, containerStatus.ContainerID)
							case "CrashLoopBackOff":
								// TODO stream container_crash user-error
								processPod.SetContainerProcessed(containerStatus.Name, containerStatus.ContainerID)
							}
						}
						if containerStatus.State.Terminated != nil {
							// TODO dig more, think more.
							// TODO not necessary because in that container state we still able to reach containers logs
							// TODO what about failed state? we should stream user-error about incorrectly terminated container
							// TODO init-container should be finally terminated in normal situation
							// TODO that error should follow logs and not preceede them
							// TODO so it is needed to move that if into after send-logs-for-container section

							c.Log("Pod %s container %s (%s) is in terminated state: %s: %s", processPod.ResourceName, containerStatus.Name, containerStatus.State.Terminated.ContainerID, containerStatus.State.Terminated.Reason, containerStatus.State.Terminated.Message)
							processPod.SetContainerProcessed(containerStatus.Name, containerStatus.ContainerID)
						}
					}

					job.SchedulePodMonitoring(processPod)
					time.Sleep(time.Duration(1) * time.Second) // TODO remove this
					continue TakeNextMonitoredPod
					// TODO: fetch state and stream to feed userspace-error
				}
			}
		}

		// TODO: init-containers

	ProcessContainers:
		for _, containerName := range processPod.GetMonitoredContainersNames() {
			c.Log("Process pod %s container %s", processPod.ResourceName, containerName)
			if containerStatus := processPod.GetContainerStatus(containerName); containerStatus != nil {
				c.Log("Process pod %s container %s status: %+v", processPod.ResourceName, containerName, containerStatus)
				if containerStatus.State.Waiting != nil {
					if containerStatus.State.Waiting.Reason == "RunContainerError" {
						// TODO: stream userspace-error container_stuck to watch feed
						processPod.SetContainerProcessed(containerName, containerStatus.ContainerID)
						continue ProcessContainers
					}
				}
			} else {
				c.Log("Process pod %s container %s status not available", processPod.ResourceName, containerName)
			}

			logLines, err := processPod.GetContainerLogs(containerName)
			if err != nil {
				c.Log("Error getting job %s pod %s container %s log chunk: %s", job.ResourceName, processPod.ResourceName, containerName, err)
			}

			chunk := &JobLogChunk{
				JobName:       job.ResourceName,
				PodName:       processPod.ResourceName,
				ContainerName: containerName,
				LogLines:      logLines,
			}
			if err = job.watchFeed.WriteJobLogChunk(chunk); err != nil {
				c.Log("Error writing job %s pod %s container %s log chunk to watch feed: %s", chunk.JobName, chunk.PodName, chunk.ContainerName, err)
			}
		}

		if len(processPod.GetMonitoredContainersNames()) > 0 {
			job.SchedulePodMonitoring(processPod)
		}

		time.Sleep(time.Duration(1) * time.Second) // TODO: show logs flawlessly without any suspension if there is something to show, also use ticker
	}

	// TODO: wait till job done event
	// TODO: make refresh for pods while waiting: job.RefreshMonitoredPods()
	// TODO: it is not necessary to refresh list on every tick

	// TODO: add following event watch before ending this function
	// 	switch e.Type {
	// 	case watch.Added, watch.Modified:
	// 		c.Log("o = %v", o)
	// 		c.Log("o.Status = %v", o.Status)
	// 		c.Log("o.Status.Conditions = %v", o.Status.Conditions)
	// 		for _, c := range o.Status.Conditions {
	// 			if c.Type == batchinternal.JobComplete && c.Status == core.ConditionTrue {
	// 				return true, nil
	// 			} else if c.Type == batchinternal.JobFailed && c.Status == core.ConditionTrue {
	// 				return true, fmt.Errorf("Job failed: %s", c.Reason)
	// 			}
	// 		}
	// 		c.Log("%s: Jobs active: %d, jobs failed: %d, jobs succeeded: %d", jobInfo.Name, o.Status.Active, o.Status.Failed, o.Status.Succeeded)
	// 		return false, nil
	// 	}

	return nil
}

func (c *Client) WatchJobsTillDone(namespace string, reader io.Reader, watchFeed WatchFeed, timeout time.Duration) error {
	infos, err := c.Build(namespace, reader)
	if err != nil {
		return err
	}

	return perform(infos, func(info *resource.Info) error {
		return c.watchJobTillDone(info, watchFeed, timeout)
	})
}
