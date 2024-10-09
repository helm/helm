#!/bin/sh

# generate
openssl req -new -config openssl.conf -key key.pem -out key.csr -addext "subjectAltName = DNS:helm.sh, IP Address:127.0.0.1"
openssl ca -config openssl.conf -rand_serial -batch -in key.csr -out crt.pem -keyfile rootca.key -cert rootca.crt

# generate localhost certificate (mainly used for http redirect tests)
openssl req -new -config openssl.conf -key key.pem -out localhost-key.csr -addext "subjectAltName = DNS:localhost"
openssl ca -config openssl.conf -rand_serial -batch -in localhost-key.csr -out localhost-crt.pem -keyfile rootca.key -cert rootca.crt
