import sys

out = "services:\n"
ips = []

for i in range(1, 10):
    port = 6370 + i
    out += f"""  redis-node-{i}:
    image: redis:7.4.2-bookworm
    command: ["redis-server", "--port", "{port}", "--cluster-enabled", "yes", "--cluster-config-file", "nodes.conf", "--cluster-node-timeout", "5000", "--appendonly", "yes", "--min-replicas-to-write", "1", "--min-replicas-max-lag", "5"]
    ports:
      - "{port}:{port}"
    volumes:
      - ../.data/redis-{i}:/data
    healthcheck:
      test: ["CMD", "redis-cli", "-p", "{port}", "ping"]
      interval: 5s
      timeout: 3s
      retries: 20

"""
    ips.append(f"redis-node-{i}:{port}")

out += f"""  redis-cluster-init:
    image: redis:7.4.2-bookworm
    depends_on:
"""
for i in range(1, 10):
    out += f"      redis-node-{i}:\n        condition: service_healthy\n"

out += "    command: >\n      sh -c 'echo \"yes\" | redis-cli --cluster create " + " ".join(ips) + " --cluster-replicas 2'\n"

print(out)
