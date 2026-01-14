# testsshd

This directory contains a simple sshd server that can be used for testing rexec.

The docker (and docker compose) and testsshd is essential for a proper 
reproduction of rexec's ssh related tests. 
If you cannot run docker on your environment, consider running testsshd
somewhere else and proxying the port 24622 to it.

## Usage

To start the server, run:

```bash
cd testsshd  # current directory

docker compose -f testsshd-docker-compose.yml up
```

The ssh service will be available at `127.0.0.1:24622`,
You can login into it with username `root` and password `root`
or with the private key `testsshd.id_rsa`.

```bash
# private key login
ssh -p 24622 -i testsshd.id_rsa root@localhost

# password login
ssh -p 24622 root@localhost
# enter password: root
```

To stop the server, run:

```bash
docker compose -f testsshd-docker-compose.yml down
```
