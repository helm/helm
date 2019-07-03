*** Settings ***
Documentation     Verify Helm functionality on multiple Kubernetes versions
Library           lib/Kind.py
Library           lib/Kubectl.py
Library           lib/Helm.py
Suite Setup       Suite Setup
Suite Teardown    Suite Teardown

*** Test Cases ***
Helm works with Kubernetes 1.14.3
    Test Helm on Kubernetes version   1.14.3

Helm works with Kubernetes 1.15.0
    Test Helm on Kubernetes version   1.15.0

*** Keyword ***
Test Helm on Kubernetes version
    [Arguments]    ${kube_version}
    Kind.Create test cluster with Kubernetes version  ${kube_version}
    Kind.Wait for cluster

    Kubectl.Get nodes
    Kubectl.Get pods    kube-system

    Helm.List releases

    kind.Delete test cluster

Suite Setup
    Kind.cleanup all test clusters

Suite Teardown
    Kind.cleanup all test clusters