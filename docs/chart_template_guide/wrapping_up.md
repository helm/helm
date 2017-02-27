# Wrapping Up

This guide is intended to give you, the chart developer, a strong understanding of how to use Helm's template language. The guide focuses on the technical aspects of template development.

But there are many things this guide has not covered when it comes to the practical day-to-day development of charts. Here are some useful pointers to other documentation that will help you as you create new charts:

- The [Kubernetes Charts project](https://github.com/kubernetes/charts) is an indispensable source of charts. That project is also sets the standard for best practices in chart development.
- The Kubernetes [User's Guide](http://kubernetes.io/docs/user-guide/) provides detailed examples of the various resource kinds that you can use, from ConfigMaps and Secrets to DaemonSets and Deployments.
- The Helm [Charts Guide](../charts.md) explains the workflow of using charts.
- The Helm [Chart Hooks Guide](../charts_hooks.md) explains how to create lifecycle hooks.
- The Helm [Charts Tips and Tricks](../charts_tips_and_tricks.md) article provides some useful tips for writing charts.
- The [Sprig documentation](https://github.com/Masterminds/sprig) documents more than sixty of the template functions.
- The [Go template docs](https://godoc.org/text/template) explain the template syntax in detail.
- The [Schelm tool](https://github.com/databus23/schelm) is a nice helper utility for debugging charts.

Sometimes it's easier to ask a few questions and get answers from experienced developers. The best place to do that is in the Kubernetes `#Helm` Slack channel:

- [Kubernetes Slack](https://slack.k8s.io/): `#helm`

Finally, if you find errors or omissions in this document, want to suggest some new content, or would like to contribute, visit [The Helm Project](https://github.com/kubernetes/helm).
