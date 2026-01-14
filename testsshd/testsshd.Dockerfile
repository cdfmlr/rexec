# fork from https://github.com/testcontainers/sshd-docker/blob/main/Dockerfile
# add public key support

FROM alpine:3.19.1
RUN apk add --no-cache openssh && ssh-keygen -A

# ENV USERNAME="root"
ENV PASSWORD="root"

COPY ./testsshd.id_rsa.pub testsshd.id_rsa.pub

RUN mkdir -p /root/.ssh && \
    cat testsshd.id_rsa.pub >> /root/.ssh/authorized_keys && \
    chmod 600 /root/.ssh/authorized_keys && \
    chmod 700 /root/.ssh

ENTRYPOINT ["sh", "-c"]
CMD ["echo root:${PASSWORD} | chpasswd && /usr/sbin/sshd -D -o PermitRootLogin=yes -o AddressFamily=inet -o GatewayPorts=yes -o AllowAgentForwarding=yes -o AllowTcpForwarding=yes -o KexAlgorithms=+diffie-hellman-group1-sha1 -o HostkeyAlgorithms=+ssh-rsa "]
