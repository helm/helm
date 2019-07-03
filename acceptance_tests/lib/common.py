import os
import subprocess
import time

NOW = time.strftime('%Y%m%d%H%M%S')

class CommandRunner(object):
    def __init__(self):
        self.rc = 0
        self.pid = 0
        self.stdout = ''
        self.rootdir = os.path.realpath(os.path.join(__file__, '../../../'))

    def return_code_should_be(self, expected_rc):
        if int(expected_rc) != self.rc:
            raise AssertionError('Expected return code to be "%s" but was "%s".'
                                 % (expected_rc, self.rc))

    def return_code_should_not_be(self, expected_rc):
        if int(expected_rc) == self.rc:
            raise AssertionError('Expected return code not to be "%s".' % expected_rc)

    def output_contains(self, s):
        if s not in self.stdout:
            raise AssertionError('Output does not contain "%s".' % s)

    def output_does_not_contain(self, s):
        if s in self.stdout:
            raise AssertionError('Output contains "%s".' % s)

    def run_command(self, command, detach=False):
        process = subprocess.Popen(['/bin/bash', '-xc', command],
                                   stdout=subprocess.PIPE,
                                   stderr=subprocess.STDOUT)
        if not detach:
            stdout = process.communicate()[0].strip().decode()
            self.rc = process.returncode
            tmp = []
            for x in stdout.split('\n'):
                print(x)
                if not x.startswith('+ '): # Remove debug lines that start with "+ "
                    tmp.append(x)
            self.stdout = '\n'.join(tmp)
