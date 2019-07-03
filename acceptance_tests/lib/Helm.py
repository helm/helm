import os
import common
from Kind import kind_auth_wrap

TEST_CHARTS_ROOT_DIR = os.path.abspath(os.path.dirname(os.path.realpath(__file__)) +'/../testdata/charts')

class Helm(common.CommandRunner):
    def list_releases(self):
        cmd = 'helm list'
        self.run_command(kind_auth_wrap(cmd))

    def install_test_chart(self, release_name, test_chart, extra_args):
        chart_path = TEST_CHARTS_ROOT_DIR+'/'+test_chart
        cmd = 'helm install '+release_name+' '+chart_path+' '+extra_args
        self.run_command(kind_auth_wrap(cmd))

    def delete_release(self, release_name):
        cmd = 'helm delete '+release_name
        self.run_command(kind_auth_wrap(cmd))
