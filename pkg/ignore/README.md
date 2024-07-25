## HIP-0019 helm lint ignore file

### manual test

```bash
go run ./cmd/helm lint ~/repositories/gitlab/chart/ --lint-ignore-file ~/repositories/gitlab/chart/.helmlintignore --with-subcharts --debug
```

```
go run ./cmd/helm lint ../gitlab/chart/ --lint-ignore-file ../gitlab/chart/.helmlintignore --with-subcharts --debug
```

### code flow diagram

```mermaid
flowchart LR
    classDef lintIgnores fill:#f9f,stroke:#333,stroke-width:4px;
    
subgraph main["package main"]
    Filter:::lintIgnores
    
    root --> cmdHelmLint
    cmdHelmLint[cmd/helm/lint.go] --> action
    cmdHelmLint[cmd/helm/lint.go] --> lint/rules
    cmdHelmLint[cmd/helm/lint.go] --> lint/support
    cmdHelmLint --> Filter["FilterIgnoredMessages()"]
    cmdHelmLint --> action
end

subgraph action["package action"]
    action --> aNewLint["action.NewLint()"]
    action --> typeLint["type action.Lint"]
    action --> typeLintResult["type action.LintResult"]
end

subgraph lint["package lint"]
    subgraph support["package lint/support"]
        lint/support
        lint/support --> Message["type support.Message"]
    end
    
    subgraph rules
        parseIgnore:::lintIgnores
        lint/rules
        lint/rules --> parseIgnore["rules.ParseIgnoreFile()"]
    end
    
end

    

```