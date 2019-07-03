import common
from Kind import kind_auth_wrap

class Kubectl(common.CommandRunner):
    def get_nodes(self):
        cmd = 'kubectl get nodes'
        self.run_command(kind_auth_wrap(cmd))

    def get_pods(self, namespace):
        cmd = 'kubectl get pods --namespace='+namespace
        self.run_command(kind_auth_wrap(cmd))
