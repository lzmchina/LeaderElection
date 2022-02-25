FROM golang:1.16 AS pre
COPY . /codes
RUN cd /codes && go build -o leaderElector 

FROM  ubuntu
COPY --from=pre /codes/leaderElector /usr/local/bin/
ENTRYPOINT ["leaderElector", "--id=$(hostname)"]