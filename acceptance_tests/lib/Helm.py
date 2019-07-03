import common
from Kind import kind_auth_wrap

class Helm(common.CommandRunner):
    def list_releases(self):
        cmd = 'helm list'
        self.run_command(kind_auth_wrap(cmd))
