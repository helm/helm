import common
from Kind import kind_auth_wrap

class Kubectl(common.CommandRunner):
    def get_nodes(self):
        cmd = 'kubectl get nodes'
        self.run_command(kind_auth_wrap(cmd))

    def get_pods(self, namespace):
        cmd = 'kubectl get pods --namespace='+namespace
        self.run_command(kind_auth_wrap(cmd))

    def get_services(self, namespace):
        cmd = 'kubectl get services --namespace='+namespace
        self.run_command(kind_auth_wrap(cmd))

    def get_persistent_volume_claims(self, namespace):
        cmd = 'kubectl get pvc --namespace='+namespace
        self.run_command(kind_auth_wrap(cmd))

    def service_has_ip(self, namespace, service_name):
        cmd = 'kubectl get services --namespace='+namespace
        cmd += ' | grep '+service_name
        cmd += ' | awk \'{print $3}\' | grep \'\(.\).*\\1\''
        self.run_command(kind_auth_wrap(cmd))

    def persistent_volume_claim_is_bound(self, namespace, pvc_name):
        cmd = 'kubectl get pvc --namespace='+namespace
        cmd += ' | grep '+pvc_name
        cmd += ' | awk \'{print $2}\' | grep ^Bound'
        self.run_command(kind_auth_wrap(cmd))

    def pods_with_prefix_are_running(self, namespace, pod_prefix, num_expected):
        cmd = '[ `kubectl get pods --namespace='+namespace
        cmd += ' | grep ^'+pod_prefix+' | awk \'{print $2 "--" $3}\''
        cmd += ' | grep -E "^([1-9][0-9]*)/\\1--Running" | wc -l` == '+num_expected+' ]'
        self.run_command(kind_auth_wrap(cmd))