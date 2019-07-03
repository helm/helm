*** Settings ***
Documentation     Verify Helm functionality on multiple Kubernetes versions
Library           String
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
    Create test cluster with kube version    ${kube_version}
    Verify wait flag works
    Kind.Delete test cluster

Create test cluster with kube version
    [Arguments]    ${kube_version}
    Kind.Create test cluster with Kubernetes version  ${kube_version}
    Kind.Wait for cluster
    Kubectl.Get nodes
    Kubectl.return code should be  0
    Kubectl.Get pods    kube-system
    Kubectl.return code should be  0

Verify wait flag works
    # Install nginx chart in a good state, using --wait flag
    Helm.Delete release    wait-flag-good
    Helm.Install test chart    wait-flag-good    nginx   --wait --timeout=60s
    Helm.return code should be  0

    # Make sure everything is up-and-running
    # TODO

    # Delete good release
    Helm.Delete release    wait-flag-good
    Helm.return code should be  0

    # Install nginx chart in a bad state, using --wait flag
    Helm.Delete release    wait-flag-bad
    Helm.Install test chart    wait-flag-bad   nginx   --wait --timeout=60s --set breakme=true

    # Install should return non-zero, as things fail to come up
    Helm.return code should not be  0

    # Delete bad release
    Helm.Delete release    wait-flag-bad
    Helm.return code should be  0

Suite Setup
    Kind.cleanup all test clusters

Suite Teardown
    Kind.cleanup all test clusters
