Collection of convenience scripts
=================================

[Vagrantfile](Vagrantfile)
--------------------------

A Vagrantfile to create a standalone build environment for helm.
It is handy if you do not have Golang and the dependencies used by Helm on your local machine.

    $ git clone https://github.com/kubernetes/deployment-manager.git
    $ cd deployment-manager/hack
    $ vagrant up

Once the machine is up, you can SSH to it and start a new build of helm

    $ vagrant ssh
    $ cd src/github.com/kubernetes/deployment-manager
    $ make build

[dm-push.sh](dm-push.sh)
------------------------

Run this from deployment-manager root to build and push the dm client plus
kubernetes install config into the publicly readable GCS bucket gs://get-dm.
