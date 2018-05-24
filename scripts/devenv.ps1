$pwd = (Get-Location).Path

docker build --pull -t helm-devenv $pwd/scripts 
docker run -it -v ${pwd}:/usr/local/go/src/k8s.io/helm  -w /usr/local/go/src/k8s.io/helm helm-devenv /bin/bash