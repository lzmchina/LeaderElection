FROM golang:1.16 AS pre
COPY . /codes
RUN cd /codes && go build -o leaderElection

FROM  ubuntu
COPY --from=pre /codes/leaderElection /usr/local/bin/