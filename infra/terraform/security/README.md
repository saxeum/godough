# Security stack

Account-level controls that belong to neither the VPC (`data`) nor the cluster
(`app`). Today it is one resource: the GuardDuty detector.

```
cp backend.hcl.example backend.hcl
# set bucket to the #102 output
terraform init -backend-config=backend.hcl
terraform plan -out=tfplan
terraform apply tfplan
```

## Why a separate stack

`data` is the network and the database, `eks` is the cluster, `app` is what runs in
it. An account-wide detector has no dependency on any of them, and putting it in one
of them would make every `data` deploy an opportunity to disable the alarm. It also
means the blast radius of a bad `app` apply cannot reach the thing that watches the
account (#123 wants CI split by area; this stack is one of those areas).

## What enabling it costs

GuardDuty bills per volume of events analysed, not per seat: at staging traffic it
is a few dollars a month. Say what the first bill was in the ticket comment so the
next person does not have to guess.

## What it does not do

No Security Hub and no Config recorder. Both are larger commitments — findings
aggregation across services, and a rule per resource type with its own cost — and
neither is worth doing before the detector has been running long enough to show
what it actually reports here.
