*** Settings ***
Documentation     Verify Helm functionality on multiple Kubernetes versions.
...
...               Fresh new kind-based clusters will be created for each
...               of the Kubernetes versions being tested. An existing
...               kind cluster can be used by specifying it in an env var
...               representing the version, for example:
...
...                  export KIND_CLUSTER_1_14_3="helm-ac-keepalive-1.14.3"
...                  export KIND_CLUSTER_1_15_0="helm-ac-keepalive-1.15.0"
...
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

    # Add new test cases here
    Verify --wait flag works as expected

    Kind.Delete test cluster

Create test cluster with kube version
    [Arguments]    ${kube_version}
    Kind.Create test cluster with Kubernetes version  ${kube_version}
    Kind.Wait for cluster
    Kubectl.Get nodes
    Kubectl.Return code should be  0
    Kubectl.Get pods    kube-system
    Kubectl.Return code should be  0

Verify --wait flag works as expected
    # Install nginx chart in a good state, using --wait flag
    Helm.Delete release    wait-flag-good
    Helm.Install test chart    wait-flag-good    nginx   --wait --timeout=60s
    Helm.Return code should be  0

    # Make sure everything is up-and-running
    Kubectl.Get pods    default
    Kubectl.Get services    default
    Kubectl.Get persistent volume claims    default

    Kubectl.Service has IP  default    wait-flag-good-nginx
    Kubectl.Return code should be   0

    Kubectl.Persistent volume claim is bound    default    wait-flag-good-nginx
    Kubectl.Return code should be   0

    Kubectl.Pods with prefix are running    default    wait-flag-good-nginx-ext-    3
    Kubectl.Return code should be   0
    Kubectl.Pods with prefix are running    default    wait-flag-good-nginx-fluentd-es-    1
    Kubectl.Return code should be   0
    Kubectl.Pods with prefix are running    default    wait-flag-good-nginx-v1-    3
    Kubectl.Return code should be   0
    Kubectl.Pods with prefix are running    default    wait-flag-good-nginx-v1beta1-    3
    Kubectl.Return code should be   0
    Kubectl.Pods with prefix are running    default    wait-flag-good-nginx-v1beta2-    3
    Kubectl.Return code should be   0
    Kubectl.Pods with prefix are running    default    wait-flag-good-nginx-web-   3
    Kubectl.Return code should be   0

    # Delete good release
    Helm.Delete release    wait-flag-good
    Helm.Return code should be  0

    # Install nginx chart in a bad state, using --wait flag
    Helm.Delete release    wait-flag-bad
    Helm.Install test chart    wait-flag-bad   nginx   --wait --timeout=60s --set breakme=true

    # Install should return non-zero, as things fail to come up
    Helm.Return code should not be  0

    # Make sure things are NOT up-and-running
    Kubectl.Get pods    default
    Kubectl.Get services    default
    Kubectl.Get persistent volume claims    default

    Kubectl.Persistent volume claim is bound    default    wait-flag-bad-nginx
    Kubectl.Return code should not be   0

    Kubectl.Pods with prefix are running    default    wait-flag-bad-nginx-ext-    3
    Kubectl.Return code should not be   0
    Kubectl.Pods with prefix are running    default    wait-flag-bad-nginx-fluentd-es-    1
    Kubectl.Return code should not be   0
    Kubectl.Pods with prefix are running    default    wait-flag-bad-nginx-v1-    3
    Kubectl.Return code should not be   0
    Kubectl.Pods with prefix are running    default    wait-flag-bad-nginx-v1beta1-    3
    Kubectl.Return code should not be   0
    Kubectl.Pods with prefix are running    default    wait-flag-bad-nginx-v1beta2-    3
    Kubectl.Return code should not be   0
    Kubectl.Pods with prefix are running    default    wait-flag-bad-nginx-web-   3
    Kubectl.Return code should not be   0

    # Delete bad release
    Helm.Delete release    wait-flag-bad
    Helm.Return code should be  0

Suite Setup
    Kind.Cleanup all test clusters

Suite Teardown
    Kind.Cleanup all test clusters
