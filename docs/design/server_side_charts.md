# Supporting charts on the server side
As discussed in #367, the server side components manager and resourcifier have not yet been refactored, as of this writing, to work with [the new chart format](docs/design/chart_format.md). Presumably, expandybird should not be affected.

## Context
In discussions about the refactoring on the Slack channel, a number of potential requirements were discussed that could complicate matters, such as:
* supporting multiple templating technologies within a single chart.
* supporting multiple templating technologies within a single dependency graph.
* supporting the parameterization of charts, as opposed to templates.
* supporting more than one top level template in a single chart.
* supporting dependency definition and resolution mechanism at the chart level.

It was also observed that there is strong motivation to complete the first pass of the helm/dm integration, using the existing DM templating engine, before adding new features or addressing more complex scenarios.

## Assertions
In view of this context, commentators asserted that:
* It seems unlikely for a single chart to require more than one generation technology.
* It seems unnecessary that we need to support more than one top level template per chart, if templates can nest, since any forest of top level templates required by the user can be implemented using a tree, where a single top level template calls the members of the forest.

If we accept these assertions, then parameterizing a chart becomes the same thing as parameterizing a single top level template. This is an important result, because it means that:
* we don't need to rationalize schemas, type systems or parameterization mechanisms across multiple technologies.
* we don't need to worry about aggregating parameters from multiple templates in a chart level parameter block.
* we don't need to examine the intermediate results of template expansion from one templating engine to see if they need further processing by a different one.
* we can examine metadata, such as a `//helm:generate directive` in the top level template, or a property in the `Chart.yaml` file, to identify the templating engine for the top level template, and then hand that engine the chart, and let it chase template references and expand results all the way down to primitives.

## Implications
Note, however, that these assertions require templates to nest, which in turn requires a working dependency management mechanism that supports the definition and resolution of references. 

Currently, DM templates provide such a mechanism. We resolve references iteratively during expansion (i.e., resolve references, expand, repeat). However, a reference resolves to a template file, which is assumed to be the only top level template file in the target folder. It may pull in other files from the folder, but it's the root of the file graph in that folder.

If we preserve this model of a single top level template per folder, then the process of resolving template references doesn't have to change much to use charts instead of raw folders. References can resolve to charts, which we can retrieve to extract their contents (i.e,. since they may be tarballs or zipballs).

If DM can pull files from charts, then there is no need for dependency management at the chart level, as long as we're working with DM templates. This is desirable, at least for the near term, because supporting dependency management at the chart level would require us to:
* rework the DM dependency management mechanism, and
* solve a number of problems that the DM mechanism currently solves, such as propagating parameters and expansion results down the expansion graph, for arbitrary templating technologies.

Given the trade-offs here, it seems prudent to deemphasize dependency management at the chart level, and perhaps even to deprecate/remove the mechanism proposed in the chart format document. We've had trouble in the past defining what, exactly, dependencies at the chart level should mean and how we should implement them. In the spirit of [YAGNI](https://en.wikipedia.org/wiki/You_aren%27t_gonna_need_it), we can start by removing them, and then bring them back if and only if, when and only when, we actually need them to satisfy some well defined requirement.

## Compatibility
It was also observed in discussion on the Slack channel that by focusing only on DM for the moment, we run the risk of locking in assumptions that might cause compatibility problems later. However, if we assume a single top level template per chart, then introducing new templating engines should be strictly additive, and should not require backward incompatible changes to the chart format. The templating engine that owns the top level template in a given chart can have free reign to dictate the chart content constraints and dependency management mechanism for its inputs without affecting the chart formats used by other templating engines, assuming that the overall directory structure and top level `Chart.yaml` remain intact.

Granted, delegating dependency management to the templating engines means that in order to display dependencies to the user, the client will not be able to simply examine the chart. Instead, it will have to ask the server to enumerate the dependencies, and that the server in turn will have to ask the top level templating engine the same question, and then pass back the results to the client. However, this seems like a modest price to pay for the benefit of not having to solve the general dependency management problem for arbitrary templating technologies.

## Conclusion
In the spirit of crawl, walk, run, this proposal therefore focuses on making the server side components work correctly with the new chart format, using the existing DM template formats, before adding additional features, such as supporting other template formats using the `//helm:generate` directive proposed in [the chart format document](docs/design/chart_format.md).
