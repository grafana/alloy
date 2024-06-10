set -ex

clusters="dev-eu-west-2 dev-us-central-0 dev-us-east-0 ops-eu-south-0 ops-us-east-0 prod-ap-south-0 prod-ap-south-1 prod-ap-southeast-0 prod-ap-southeast-1 prod-au-southeast-0 prod-au-southeast-1 prod-ca-east-0 prod-eu-north-0 prod-eu-west-0 prod-eu-west-2 prod-eu-west-3 prod-gb-south-0 prod-sa-east-0 prod-sa-east-1 prod-us-central-0 prod-us-central-3 prod-us-central-4 prod-us-central-5 prod-us-east-0 prod-us-east-1 prod-us-west-0 us-central2"
clusters="ops-us-east-0"
clusters="dev-us-central-0"

dst=/home/korniltsev/p/alloy/korniltsev/data/configs
rm -rf $dst
cd /home/korniltsev/p/deployment_tools/ksonnet
for c in $clusters; do
    echo $c
    tk export $dst/$c environments/pyroscope-ebpf --name environments/pyroscope-ebpf/$c.pyroscope-ebpf
done




