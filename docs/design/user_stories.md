## Appendix A: User Stories for Charts

Personas:

- Operator: Responsible for running an application in production.
- Chart Dev: Responsible for developing new charts
- App Dev: Developer who creates applications that make use of existing charts, but does not create charts.


Stories:

- As an operator, I want a deployment that is 100% reproducible (exact versions)
- As an app dev, I want to be able to search for charts using keys defined in the [chart file](#the-chart-file)...
        * by keyword, where one app may have multiple keywords (e.g. Redis has storage, message queue)
        * by name (meaning name of the chart), where name may be "fuzzy".
        * by author
        * by last updated date
- As a chart dev, I want a well-defined set of practices to follow
- As a chart dev, I want to be able to work with a team on the same chart
- As a chart dev, I want to be able to indicate when a particular chart is stable, and how stable it is
- As a chart dev, I want to indicate the role I played in building a chart
- As a chart dev, I want to be able to use all of the low-level Kubernetes kinds
- As an operator, I want to be able to determine how stable a package is
- As an operator, I want to be able to determine what version of Kubernetes I need to run a chart
- As an operator, I want to determine whether a chart requires extension kinds (e.g. DaemonSet or something custom), and determine this _before_ I try to install
- As a chart dev, I want to be able to express that my chart depends on others, even to the extent that I specify the version or version range of the other chart upon which I depend
- As a chart dev, I do not want to install additional tooling to write, test, or locally run a chart (this relates to the file format in that the format should not require additional tooling)
- As a chart dev, I want to be able to store auxiliary files of arbitrary type inside of a chart (e.g. a PDF with documentation)
- As a chart dev, I want to be able to store my chart in one repository, but reference a chart in another repository
- As a chart dev, I want to embed my template inside of the code that it references. For example, I want to have the code to build a docker image and the chart to all live in the same source code repository.



