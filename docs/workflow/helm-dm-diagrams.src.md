# Helm-DM Workflow Diagrams

# Helm Official
@startuml helm-official-workflow.png
autonumber
actor Member
actor Core
participant GitHub
participant CI
database Repository
Member->GitHub: PR New Chart
GitHub->CI: Run automated tests
CI->GitHub: Tests pass
GitHub-->Member: Tests pass
GitHub-->Core: Tests pass
Core<->Member: Discussion about chart
Core->GitHub: Code review
Core->GitHub: Release approval
GitHub->CI: Issue release
CI->Repository: Push release
Member->Repository: Get release
@enduml

# Public Chart Repository
An example workflow using Launchpad and BZR.
@startuml public-chart-repo.png
autonumber
actor Developer
actor Maintainer
participant Launchpad
Developer->Launchpad: Push branch with chart
Maintainer->Launchpad: Fetch branch
Maintainer->Maintainer: Locally test
Maintainer->Launchpad: Merge branch
... later ...
Maintainer->Launchpad: Create global release
Maintainer->Maintainer: Regenerate all versioned charts
Maintainer->Repository: Push charts
@enduml

# Private chart repository
An example workflow using an internal Gerrit Git repo and Jenkins.
@startuml private-chart-repo.png
autonumber
actor "Developer A" as A
actor "Developer B" as B
actor "Internal user" as Internal
participant Gerrit
participant Jenkins
participant "Docker registry" as Docker
participant "Chart repository" as Chart
A->Gerrit: Clone repo
A->A: Local development
A->Gerrit: Push branch
Gerrit->Jenkins: Test branch
Jenkins->Jenkins: Build and test
Jenkins->Docker: Snapshot image push
Jenkins->Chart: Snapshot chart
Jenkins->Jenkins: Integration tests
Jenkins-->A: Build OK
B->Gerrit: Code review
B->A: Code approval
A->Gerrit: Tag the new chart
Gerrit->Jenkins: Release
Jenkins->Jenkins: Build and test
Jenkins->Docker: Production image push
Jenkins->Chart: Production chart
Internal->Chart: Get production chart
@enduml

# Private Charts without repository
An example using SVN for local dev, and no chart repository.
@startuml private-chart-no-repo.png
autonumber
actor "Developer A" as A
actor "Developer B" as B
participant SVN
participant Kubernetes
A->SVN: Checkout code
A->A: Develop locally
A->SVN: Check in code
B<->SVN: Update local
B->Kubernetes: Run helm deploy on local chart
@enduml
