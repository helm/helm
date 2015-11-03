FROM python:2-onbuild

RUN ln -s /usr/local/bin/python /usr/bin/python

RUN mkdir -p /var/expandybird/expansion
WORKDIR /var/expandybird

ADD expandybird ./expandybird
ADD expansion ./expansion

ADD requirements.txt ./requirements.txt
RUN pip install -r ./requirements.txt

EXPOSE 8080

ENTRYPOINT ["./expandybird", "-expansion_binary", "./expansion/expansion.py"]
