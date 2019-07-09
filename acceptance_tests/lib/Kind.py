import common
import time
import os

DOCKER_HUB_REPO='kindest/node'
CLUSTER_PREFIX = 'helm-acceptance-test'
LOG_LEVEL = 'debug'

MAX_WAIT_KIND_NODE_SECONDS = 60
KIND_NODE_INTERVAL_SECONDS = 2

MAX_WAIT_KIND_POD_SECONDS = 60
KIND_POD_INTERVAL_SECONDS = 2

KIND_POD_EXPECTED_NUMBER = 8

LAST_CLUSTER_NAME = 'UNSET'
LAST_CLUSTER_EXISTING = False

def kind_auth_wrap(cmd):
    c = 'export KUBECONFIG="$(kind get kubeconfig-path'
    c += ' --name="'+LAST_CLUSTER_NAME+'")"'
    return c+' && '+cmd

class Kind(common.CommandRunner):
    def create_test_cluster_with_kubernetes_version(self, kube_version):
        global LAST_CLUSTER_NAME, LAST_CLUSTER_EXISTING
        existing_cluster_name = os.getenv('KIND_CLUSTER_'+kube_version.replace('.', '_'))
        if existing_cluster_name:
            print('Using existing kind cluster for '+kube_version+', "'+existing_cluster_name+'"')
            LAST_CLUSTER_NAME = existing_cluster_name
            LAST_CLUSTER_EXISTING = True
        else:
            new_cluster_name = CLUSTER_PREFIX+'-'+common.NOW+'-'+kube_version
            print('Creating new kind cluster for '+kube_version+', "'+new_cluster_name+'"')
            LAST_CLUSTER_NAME = new_cluster_name
            cmd = 'kind create cluster --loglevel='+LOG_LEVEL
            cmd += ' --name='+new_cluster_name
            cmd += ' --image='+DOCKER_HUB_REPO+':v'+kube_version
            self.run_command(cmd)

    def delete_test_cluster(self):
        if LAST_CLUSTER_EXISTING:
            print('Not deleting cluster (cluster existed prior to test run)')
        else:
            cmd = 'kind delete cluster --loglevel='+LOG_LEVEL
            cmd += ' --name='+LAST_CLUSTER_NAME
            self.run_command(cmd)

    def cleanup_all_test_clusters(self):
        cmd = 'for i in `kind get clusters| grep ^'+CLUSTER_PREFIX+'-'+common.NOW+'`;'
        cmd += ' do kind delete cluster --loglevel='+LOG_LEVEL+' --name=$i || true; done'
        self.run_command(cmd)

    def wait_for_cluster(self):
        seconds_waited = 0
        while True:
            cmd = 'kubectl get nodes | tail -n1 | awk \'{print $2}\''
            self.run_command('set +x && '+kind_auth_wrap(cmd))
            status = self.stdout.replace('\n', '').strip()
            print('Cluster node status: '+status)
            if status == 'Ready':
                break
            if MAX_WAIT_KIND_NODE_SECONDS <= seconds_waited:
                raise Exception('Max time ('+str(MAX_WAIT_KIND_NODE_SECONDS)+') reached waiting for cluster node')
            time.sleep(KIND_NODE_INTERVAL_SECONDS)
            seconds_waited += KIND_NODE_INTERVAL_SECONDS

        seconds_waited = 0
        while True:
            cmd = 'kubectl get pods -n kube-system | grep \'1\/1\' | wc -l'
            self.run_command('set +x && '+kind_auth_wrap(cmd))
            num_ready = int(self.stdout.replace('\n', '').strip())
            print('Num pods ready: '+str(num_ready)+'/'+str(KIND_POD_EXPECTED_NUMBER))
            if KIND_POD_EXPECTED_NUMBER <= num_ready:
                break
            if MAX_WAIT_KIND_POD_SECONDS <= seconds_waited:
                raise Exception('Max time ('+str(MAX_WAIT_KIND_POD_SECONDS)+') reached waiting for kube-system pods')
            time.sleep(KIND_POD_INTERVAL_SECONDS)
            seconds_waited += KIND_POD_INTERVAL_SECONDS
