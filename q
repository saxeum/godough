[33m81bfda4[m[33m ([m[1;36mHEAD[m[33m -> [m[1;32mmaster[m[33m)[m HEAD@{2026-11-27 13:10:57 +1000}: pull: Merge made by the 'ort' strategy.
[33md7e3ad7[m HEAD@{2026-11-27 13:10:52 +1000}: checkout: moving from feat/145-no-store to master
[33mf055724[m[33m ([m[1;31morigin/feat/145-no-store[m[33m, [m[1;32mfeat/145-no-store[m[33m)[m HEAD@{2026-11-27 12:54:02 +1000}: commit: Mark every API response no-store
[33m0fdbc30[m HEAD@{2026-11-27 12:53:21 +1000}: checkout: moving from master to feat/145-no-store
[33md7e3ad7[m HEAD@{2026-11-27 12:51:32 +1000}: commit: Document the network and delivery flow with six diagrams
[33m0fdbc30[m HEAD@{2026-11-27 12:50:51 +1000}: pull --ff-only: Fast-forward
[33m25a064e[m[33m ([m[1;32mdocs/network-and-delivery-flow[m[33m)[m HEAD@{2026-11-27 12:50:48 +1000}: checkout: moving from infra/167-chart-version-bump to master
[33mc10eaa4[m[33m ([m[1;31morigin/infra/167-chart-version-bump[m[33m, [m[1;32minfra/167-chart-version-bump[m[33m)[m HEAD@{2026-11-27 12:48:26 +1000}: commit: Bump the charts so a template change can reach the cluster
[33m25a064e[m[33m ([m[1;32mdocs/network-and-delivery-flow[m[33m)[m HEAD@{2026-11-27 12:48:21 +1000}: checkout: moving from docs/network-and-delivery-flow to infra/167-chart-version-bump
[33m25a064e[m[33m ([m[1;32mdocs/network-and-delivery-flow[m[33m)[m HEAD@{2026-11-27 12:46:36 +1000}: checkout: moving from master to docs/network-and-delivery-flow
[33m25a064e[m[33m ([m[1;32mdocs/network-and-delivery-flow[m[33m)[m HEAD@{2026-11-27 12:42:58 +1000}: pull --ff-only: Fast-forward
[33m0dae271[m HEAD@{2026-11-27 12:42:55 +1000}: checkout: moving from infra/144-alb-tls-policy to master
[33m8575ce4[m[33m ([m[1;32minfra/144-alb-tls-policy[m[33m)[m HEAD@{2026-11-27 12:32:15 +1000}: commit: Refuse TLS 1.0 and 1.1 at the ALB, and take 1.3
[33mcdb84d4[m HEAD@{2026-11-27 12:32:01 +1000}: checkout: moving from docs/security-review to infra/144-alb-tls-policy
[33m065ffc0[m[33m ([m[1;32mdocs/security-review[m[33m)[m HEAD@{2026-11-27 12:22:50 +1000}: commit: Record the security review and the work it implies
[33mab84f39[m[33m ([m[1;32mops/116-tlscheck-in-image[m[33m)[m HEAD@{2026-11-27 12:22:07 +1000}: checkout: moving from ops/116-tlscheck-in-image to docs/security-review
[33mab84f39[m[33m ([m[1;32mops/116-tlscheck-in-image[m[33m)[m HEAD@{2026-11-27 12:07:45 +1000}: commit: Ship tlscheck in the image, and stop sending pool parameters to the server
[33m655d47e[m HEAD@{2026-11-27 12:03:58 +1000}: checkout: moving from infra/126-lbc to ops/116-tlscheck-in-image
[33ma861a6d[m[33m ([m[1;32minfra/126-lbc[m[33m)[m HEAD@{2026-11-27 11:59:24 +1000}: commit: Run migrations as a pre-upgrade hook in the api release
[33m6374eb9[m HEAD@{2026-11-27 10:48:35 +1000}: commit: Wire sign-in for the edge, and give the node group room to roll
[33mda0f1ff[m HEAD@{2026-11-27 10:36:04 +1000}: commit: Create the ALB from two Ingresses, and point the hostname at it
[33m3d014e2[m HEAD@{2026-11-27 10:31:01 +1000}: rebase (finish): returning to refs/heads/infra/126-lbc
[33m3d014e2[m HEAD@{2026-11-27 10:31:01 +1000}: rebase (pick): Install the load balancer controller and tag the subnets it looks in
[33m8802286[m HEAD@{2026-11-27 10:31:01 +1000}: rebase (start): checkout origin/master
[33mdfffcb5[m HEAD@{2026-11-27 10:31:01 +1000}: checkout: moving from infra/126-lbc to infra/126-lbc
[33mdfffcb5[m HEAD@{2026-11-27 10:25:51 +1000}: commit: Install the load balancer controller and tag the subnets it looks in
[33mf753a0a[m HEAD@{2026-11-27 10:25:01 +1000}: checkout: moving from fix/125-repo-name-and-fullname to infra/126-lbc
[33m2bdefd7[m[33m ([m[1;32mfix/125-repo-name-and-fullname[m[33m)[m HEAD@{2026-11-27 10:19:51 +1000}: commit: Point the ui release at godough-web, and stop doubling the release name
[33mf753a0a[m HEAD@{2026-11-27 10:18:36 +1000}: checkout: moving from infra/125-helm-releases to fix/125-repo-name-and-fullname
[33m33f98f5[m[33m ([m[1;32minfra/125-helm-releases[m[33m)[m HEAD@{2026-11-27 10:09:46 +1000}: commit: Create the Secrets the charts need, and say when a tunnel is missing
[33m23b9754[m HEAD@{2026-11-27 10:09:39 +1000}: checkout: moving from infra/125-helm-releases to infra/125-helm-releases
[33m23b9754[m HEAD@{2026-11-27 10:04:54 +1000}: commit: Add the app stack that installs both charts
[33md24bfac[m HEAD@{2026-11-27 10:03:24 +1000}: checkout: moving from db/114-app-role to infra/125-helm-releases
[33mbbf0497[m[33m ([m[1;32mdb/114-app-role[m[33m)[m HEAD@{2026-11-27 10:01:06 +1000}: commit: Take the application role's password from a secret, not from the file
[33m286ebab[m HEAD@{2026-11-27 10:00:30 +1000}: checkout: moving from infra/117-amd64-nodes to db/114-app-role
[33ma46937e[m[33m ([m[1;32minfra/117-amd64-nodes[m[33m)[m HEAD@{2026-11-27 09:56:54 +1000}: commit: Run the node group on x86_64 so the images it pulls exist
[33m0dae271[m HEAD@{2026-11-27 09:45:42 +1000}: checkout: moving from ops/eks-kubeconfig to infra/117-amd64-nodes
[33mef5c900[m[33m ([m[1;32mops/eks-kubeconfig[m[33m)[m HEAD@{2026-11-27 09:42:00 +1000}: commit: Make eks-kubeconfig leave kubectl working, not a line to paste
[33m0dae271[m HEAD@{2026-11-27 09:41:03 +1000}: checkout: moving from master to ops/eks-kubeconfig
[33m0dae271[m HEAD@{2026-11-27 09:38:10 +1000}: pull: Fast-forward
[33m5afcd67[m HEAD@{2026-11-27 09:25:29 +1000}: pull: Fast-forward
[33mf9d0e43[m HEAD@{2026-11-27 09:25:24 +1000}: checkout: moving from deploy/125-helm-charts to master
[33mb59925d[m[33m ([m[1;32mdeploy/125-helm-charts[m[33m)[m HEAD@{2026-11-27 09:20:08 +1000}: commit: Add the api and ui charts, and the means to refuse a moving tag
[33m0bf9fc9[m HEAD@{2026-11-27 09:17:25 +1000}: checkout: moving from infra/117-node-sg-rules to deploy/125-helm-charts
[33m078667b[m[33m ([m[1;32minfra/117-node-sg-rules[m[33m)[m HEAD@{2026-11-27 08:58:29 +1000}: commit: Give the nodes the ingress rules the cluster SG would have given them
[33m0bf9fc9[m HEAD@{2026-11-27 08:58:11 +1000}: checkout: moving from docs/125-deploy-path to infra/117-node-sg-rules
[33md6c5947[m[33m ([m[1;32mdocs/125-deploy-path[m[33m)[m HEAD@{2026-11-27 08:55:21 +1000}: commit: Correct where deploys are applied: Terraform and Argo, not a laptop
[33m0bf9fc9[m HEAD@{2026-11-27 08:55:21 +1000}: checkout: moving from infra/121-endpoint-from-bastion to docs/125-deploy-path
[33m4ab9cb3[m[33m ([m[1;32minfra/121-endpoint-from-bastion[m[33m)[m HEAD@{2026-11-27 08:32:11 +1000}: cherry-pick: Make the cluster reachable and give it a smoke test
[33m4768246[m HEAD@{2026-11-27 08:32:11 +1000}: checkout: moving from infra/117-eks to infra/121-endpoint-from-bastion
[33m7cdc366[m[33m ([m[1;31morigin/infra/117-eks[m[33m, [m[1;32minfra/117-eks[m[33m)[m HEAD@{2026-11-27 08:31:51 +1000}: commit: Make the cluster reachable and give it a smoke test
[33m739ea56[m HEAD@{2026-11-27 08:29:19 +1000}: commit: Record that deploys are applied locally until Argo CD
[33mb039450[m HEAD@{2026-11-27 08:27:42 +1000}: commit: Say what the bastion may and may not be for cluster access
[33m3c98727[m HEAD@{2026-11-27 08:25:00 +1000}: commit: Add the staging EKS cluster and the NAT it needs
[33mce321b3[m[33m ([m[1;32mfeat/113-harden-migrator[m[33m)[m HEAD@{2026-11-27 08:22:15 +1000}: checkout: moving from feat/113-harden-migrator to infra/117-eks
[33mce321b3[m[33m ([m[1;32mfeat/113-harden-migrator[m[33m)[m HEAD@{2026-11-27 08:18:46 +1000}: commit: Lock the runner, checksum what ran, and add -status
[33m9f99867[m HEAD@{2026-11-27 08:12:56 +1000}: checkout: moving from feat/112-migrations-in-image to feat/113-harden-migrator
[33m2837211[m[33m ([m[1;32mfeat/112-migrations-in-image[m[33m)[m HEAD@{2026-11-27 07:56:54 +1000}: commit (amend): Embed the migrations in the migrator binary
[33md2593a4[m HEAD@{2026-11-27 07:56:52 +1000}: commit: Embed the migrations in the migrator binary
[33mbea8ba4[m HEAD@{2026-11-27 07:55:34 +1000}: checkout: moving from docs/todo-aws-migrations to feat/112-migrations-in-image
[33mfb5d44b[m[33m ([m[1;32mdocs/todo-aws-migrations[m[33m)[m HEAD@{2026-11-27 07:53:58 +1000}: commit: Record that #110 merged, not that it is open
[33mb322b4c[m HEAD@{2026-11-27 07:52:44 +1000}: commit: Update the backlog for AWS, EKS and migrations
[33mbea8ba4[m HEAD@{2026-11-27 07:52:35 +1000}: checkout: moving from infra/rds-vpc to docs/todo-aws-migrations
[33m30f944c[m[33m ([m[1;32minfra/rds-vpc[m[33m)[m HEAD@{2026-11-27 07:43:33 +1000}: commit: Add make targets for the RDS SSM tunnel
[33me6bb809[m HEAD@{2026-11-27 07:41:52 +1000}: commit: Document the SSM tunnel commands in the data stack
[33mcc57ab1[m HEAD@{2026-11-27 07:40:08 +1000}: commit: Add an SSM bastion for RDS access
[33m73aed0d[m HEAD@{2026-11-27 07:33:01 +1000}: commit: Add staging VPC and private RDS Postgres
[33mf9d0e43[m HEAD@{2026-11-27 07:17:43 +1000}: checkout: moving from master to infra/rds-vpc
[33mf9d0e43[m HEAD@{2026-11-27 07:17:22 +1000}: checkout: moving from master to master
[33mf9d0e43[m HEAD@{2026-11-27 07:08:21 +1000}: pull: Fast-forward
[33m25349a8[m HEAD@{2026-11-27 07:08:16 +1000}: checkout: moving from fix/ecr-registry-default to master
[33ma8690f9[m[33m ([m[1;32mfix/ecr-registry-default[m[33m)[m HEAD@{2026-11-27 07:05:33 +1000}: commit: e2e test with ui change
[33m8bb2c38[m HEAD@{2026-11-27 07:03:05 +1000}: commit: Pull the last published master image by default
[33m2d1157c[m HEAD@{2026-11-27 07:01:45 +1000}: commit: Derive the ECR registry from AWS by default
[33m25349a8[m HEAD@{2026-11-27 07:01:25 +1000}: checkout: moving from master to fix/ecr-registry-default
[33m25349a8[m HEAD@{2026-11-27 07:01:22 +1000}: checkout: moving from master to master
[33m25349a8[m HEAD@{2026-11-27 06:59:50 +1000}: pull: Fast-forward
[33ma002717[m HEAD@{2026-11-27 06:59:45 +1000}: checkout: moving from fix/aws-oidc-thumbprint to master
[33me408fe7[m[33m ([m[1;32mfix/aws-oidc-thumbprint[m[33m)[m HEAD@{2026-11-27 06:52:14 +1000}: commit: Use GitHub's immutable OIDC subject
[33ma002717[m HEAD@{2026-11-27 06:46:09 +1000}: checkout: moving from master to fix/aws-oidc-thumbprint
[33ma002717[m HEAD@{2026-11-27 06:46:09 +1000}: pull --ff-only origin master: Fast-forward
[33mb98fcca[m HEAD@{2026-11-27 06:46:06 +1000}: checkout: moving from infra/103-ecr to master
[33mdfd1602[m[33m ([m[1;32minfra/103-ecr[m[33m)[m HEAD@{2026-11-27 06:20:08 +1000}: commit: Add scheduled AWS checks, static config gate and ECR pull test
[33m9c942b4[m HEAD@{2026-11-27 06:10:45 +1000}: commit: Add negative checks for website hosting and public repository policies
[33m4334e2f[m HEAD@{2026-11-27 06:07:59 +1000}: commit: Split AWS checks into bootstrap and ECR tests
[33mc844f5d[m HEAD@{2026-11-27 06:05:48 +1000}: commit: Add ECR repositories and the publish path
[33mb98fcca[m HEAD@{2026-11-27 06:02:41 +1000}: checkout: moving from master to infra/103-ecr
[33mb98fcca[m HEAD@{2026-11-27 06:02:41 +1000}: pull --ff-only origin master: Fast-forward
[33meed2be0[m HEAD@{2026-11-27 06:02:38 +1000}: checkout: moving from infra/102-terraform-bootstrap to master
[33mfe77fca[m[33m ([m[1;32minfra/102-terraform-bootstrap[m[33m)[m HEAD@{2026-11-27 06:00:31 +1000}: commit: Require Terraform 1.17 and AWS provider 6.7
[33mb64cb6b[m HEAD@{2026-11-27 05:56:38 +1000}: commit: Simplify the AWS bootstrap checks
[33m517c7b6[m HEAD@{2026-11-27 05:55:10 +1000}: commit: Add a positive HTTPS fetch control
[33m95f9092[m HEAD@{2026-11-27 05:52:54 +1000}: commit: Assert the state bucket is not publicly listable over HTTPS
[33m2c61600[m HEAD@{2026-11-27 05:46:41 +1000}: commit: Drop the Terraform Makefile targets
[33m54590c1[m HEAD@{2026-11-27 05:37:41 +1000}: commit: Add a live check for the private state bucket
[33m6bcb980[m HEAD@{2026-11-27 05:35:19 +1000}: commit: Bootstrap Terraform state in S3
[33meed2be0[m HEAD@{2026-11-27 05:33:12 +1000}: checkout: moving from master to infra/102-terraform-bootstrap
[33meed2be0[m HEAD@{2026-11-27 05:33:01 +1000}: pull --ff-only origin master: Fast-forward
[33m45c0a59[m HEAD@{2026-11-27 05:32:58 +1000}: checkout: moving from ops/tls-check to master
[33ma450d0f[m[33m ([m[1;32mops/tls-check[m[33m)[m HEAD@{2026-11-27 05:16:38 +1000}: cherry-pick: Add a Postgres certificate expiry check
[33m45c0a59[m HEAD@{2026-11-27 05:16:38 +1000}: checkout: moving from master to ops/tls-check
[33m45c0a59[m HEAD@{2026-11-27 05:16:38 +1000}: pull --ff-only origin master: Fast-forward
[33maca3d08[m HEAD@{2026-11-27 05:16:35 +1000}: checkout: moving from db/38-postgres-tls to master
[33m03f51b8[m[33m ([m[1;32mdb/38-postgres-tls[m[33m)[m HEAD@{2026-11-27 05:16:18 +1000}: commit: Add a Postgres certificate expiry check
[33m9411cf1[m HEAD@{2026-11-27 05:11:18 +1000}: commit: Default Postgres to verify-full outside compose
[33maca3d08[m HEAD@{2026-11-27 05:10:26 +1000}: checkout: moving from master to db/38-postgres-tls
[33maca3d08[m HEAD@{2026-11-27 05:10:26 +1000}: pull --ff-only origin master: Fast-forward
[33mbf8a074[m HEAD@{2026-11-27 05:10:23 +1000}: checkout: moving from feat/56-signin-ui to master
[33m4be983f[m[33m ([m[1;32mfeat/56-signin-ui[m[33m)[m HEAD@{2026-11-27 05:01:15 +1000}: commit: Pass local Google credentials through compose
[33m00aacc8[m HEAD@{2026-11-27 04:57:53 +1000}: commit: Add the signed-out and signed-in UI
[33mbf8a074[m HEAD@{2026-11-27 04:56:06 +1000}: checkout: moving from master to feat/56-signin-ui
[33mbf8a074[m HEAD@{2026-11-27 04:56:06 +1000}: pull --ff-only origin master: Fast-forward
[33md755c4f[m HEAD@{2026-11-27 04:56:03 +1000}: checkout: moving from feat/54-ownership to master
[33m72ff817[m[33m ([m[1;32mfeat/54-ownership[m[33m)[m HEAD@{2026-11-27 04:52:54 +1000}: commit: Scope snippets to their owner
[33md755c4f[m HEAD@{2026-11-27 04:50:46 +1000}: checkout: moving from master to feat/54-ownership
[33md755c4f[m HEAD@{2026-11-27 04:50:46 +1000}: pull --ff-only origin master: Fast-forward
[33m55d64ff[m HEAD@{2026-11-27 04:50:42 +1000}: checkout: moving from auth/53-google-flow to master
[33m444ea29[m[33m ([m[1;32mauth/53-google-flow[m[33m)[m HEAD@{2026-11-27 04:44:46 +1000}: commit: Upgrade go-jose to 4.1.6
[33md0847b6[m HEAD@{2026-11-26 20:22:47 +1000}: checkout: moving from auth/52-55-sessions-csrf to auth/53-google-flow
[33md0847b6[m HEAD@{2026-11-26 20:22:42 +1000}: commit: Add the Google authorization-code flow and test seam
[33m16ec460[m HEAD@{2026-11-26 20:12:21 +1000}: commit: Add database sessions and CSRF for snippet writes
[33m55d64ff[m HEAD@{2026-11-26 20:06:38 +1000}: checkout: moving from master to auth/52-55-sessions-csrf
[33m55d64ff[m HEAD@{2026-11-26 20:05:13 +1000}: pull --ff-only origin master: Fast-forward
[33medb0055[m HEAD@{2026-11-26 20:05:10 +1000}: checkout: moving from obs/44-logs to master
[33m1945925[m[33m ([m[1;32mobs/44-logs[m[33m)[m HEAD@{2026-11-26 19:58:06 +1000}: commit: Make the request path observable
[33medb0055[m HEAD@{2026-11-26 19:55:43 +1000}: checkout: moving from master to obs/44-logs
[33medb0055[m HEAD@{2026-11-26 19:55:16 +1000}: pull --ff-only origin master: Fast-forward
[33m58a7d53[m HEAD@{2026-11-26 19:55:13 +1000}: checkout: moving from ci/47-gates to master
[33m28aebae[m[33m ([m[1;32mci/47-gates[m[33m)[m HEAD@{2026-11-26 19:52:08 +1000}: commit: Wire the gates the tree already assumes
[33m58a7d53[m HEAD@{2026-11-26 19:49:31 +1000}: checkout: moving from master to ci/47-gates
[33m58a7d53[m HEAD@{2026-11-26 19:49:28 +1000}: checkout: moving from master to master
[33m58a7d53[m HEAD@{2026-11-26 19:47:58 +1000}: pull --ff-only origin master: Fast-forward
[33m06a2737[m HEAD@{2026-11-26 19:47:55 +1000}: checkout: moving from test/45-test-database to master
[33m944cff8[m[33m ([m[1;32mtest/45-test-database[m[33m)[m HEAD@{2026-11-26 19:08:01 +1000}: commit: Give the tests their own database
[33m6942eb3[m HEAD@{2026-11-26 19:08:01 +1000}: commit: Make the backlog platform-first
[33m06a2737[m HEAD@{2026-11-26 19:04:35 +1000}: checkout: moving from master to test/45-test-database
[33m06a2737[m HEAD@{2026-11-26 18:58:59 +1000}: pull --ff-only origin master: Fast-forward
[33mbdc2c0a[m HEAD@{2026-11-26 18:58:56 +1000}: checkout: moving from fix/34-probe-routing to master
[33me94667b[m[33m ([m[1;32mfix/34-probe-routing[m[33m)[m HEAD@{2026-11-26 18:55:25 +1000}: checkout: moving from docs/prioritised-backlog to fix/34-probe-routing
[33mea7ea4c[m[33m ([m[1;32mdocs/prioritised-backlog[m[33m)[m HEAD@{2026-11-26 18:55:21 +1000}: commit: Correct the probe priority after the ALB check
[33m05e98d6[m HEAD@{2026-11-26 18:55:21 +1000}: checkout: moving from fix/34-probe-routing to docs/prioritised-backlog
[33me94667b[m[33m ([m[1;32mfix/34-probe-routing[m[33m)[m HEAD@{2026-11-26 18:55:14 +1000}: commit: Keep the health probes on the API pod
[33m88807b1[m HEAD@{2026-11-26 18:47:41 +1000}: checkout: moving from docs/prioritised-backlog to fix/34-probe-routing
[33m05e98d6[m HEAD@{2026-11-26 18:44:04 +1000}: commit: Add the prioritised backlog plan
[33m88807b1[m HEAD@{2026-11-26 18:43:53 +1000}: checkout: moving from feat/89-stylesheet to docs/prioritised-backlog
[33m767a11a[m[33m ([m[1;32mfeat/89-stylesheet[m[33m)[m HEAD@{2026-11-26 18:26:33 +1000}: commit: Replace the Snippetbox stylesheet
[33mbdc2c0a[m HEAD@{2026-11-26 18:25:03 +1000}: checkout: moving from master to feat/89-stylesheet
[33mbdc2c0a[m HEAD@{2026-11-26 18:25:03 +1000}: checkout: moving from test/74-boot-deadline to master
[33mab08d5f[m[33m ([m[1;32mtest/74-boot-deadline[m[33m)[m HEAD@{2026-11-26 18:19:48 +1000}: commit: Test the boot deadline that #62 promised
[33mbdc2c0a[m HEAD@{2026-11-26 18:18:30 +1000}: checkout: moving from master to test/74-boot-deadline
[33mbdc2c0a[m HEAD@{2026-11-26 18:18:30 +1000}: checkout: moving from master to master
[33mbdc2c0a[m HEAD@{2026-11-26 18:15:04 +1000}: checkout: moving from feat/14-migrator to master
[33m13b4a8d[m[33m ([m[1;32mfeat/14-migrator[m[33m)[m HEAD@{2026-11-26 18:11:18 +1000}: commit: Apply pending migrations instead of one hardcoded file
[33mbdc2c0a[m HEAD@{2026-11-26 18:09:53 +1000}: checkout: moving from master to feat/14-migrator
[33mbdc2c0a[m HEAD@{2026-11-26 18:09:53 +1000}: merge origin/master: Fast-forward
[33mffdf5a0[m HEAD@{2026-11-26 18:09:53 +1000}: checkout: moving from fix/39-app-role-bootstrap to master
[33m5c0022b[m[33m ([m[1;32mfix/39-app-role-bootstrap[m[33m)[m HEAD@{2026-11-26 18:04:38 +1000}: commit: Make the application role a documented bootstrap, not a Docker hook
[33mffdf5a0[m HEAD@{2026-11-26 18:03:51 +1000}: checkout: moving from master to fix/39-app-role-bootstrap
[33mffdf5a0[m HEAD@{2026-11-26 18:03:51 +1000}: merge origin/master: Fast-forward
[33m52d9285[m HEAD@{2026-11-26 18:03:51 +1000}: checkout: moving from fix/84-timeout-ladder to master
[33mb4035b4[m[33m ([m[1;32mfix/84-timeout-ladder[m[33m)[m HEAD@{2026-11-26 18:01:05 +1000}: commit: Make the drain outlast the request it is draining
[33m52d9285[m HEAD@{2026-11-26 18:00:32 +1000}: checkout: moving from master to fix/84-timeout-ladder
[33m52d9285[m HEAD@{2026-11-26 18:00:32 +1000}: checkout: moving from master to master
[33m52d9285[m HEAD@{2026-11-26 17:58:12 +1000}: merge origin/master: Fast-forward
[33mabd05bd[m HEAD@{2026-11-26 17:58:12 +1000}: checkout: moving from refactor/73-naming to master
[33me08d500[m[33m ([m[1;32mrefactor/73-naming[m[33m)[m HEAD@{2026-11-26 17:55:24 +1000}: commit: Drop the package stutter from the service type
[33mabd05bd[m HEAD@{2026-11-26 17:55:08 +1000}: checkout: moving from master to refactor/73-naming
[33mabd05bd[m HEAD@{2026-11-26 17:55:08 +1000}: merge origin/master: Fast-forward
[33m448657f[m HEAD@{2026-11-26 17:55:08 +1000}: checkout: moving from fix/70-record-method-not-allowed to master
[33m100b075[m[33m ([m[1;32mfix/70-record-method-not-allowed[m[33m)[m HEAD@{2026-11-26 17:52:32 +1000}: commit: Record the 405 as a documented exception instead of shaping it
[33m448657f[m HEAD@{2026-11-26 17:52:16 +1000}: checkout: moving from master to fix/70-record-method-not-allowed
[33m448657f[m HEAD@{2026-11-26 17:52:16 +1000}: merge origin/master: Fast-forward
[33m8ba3e07[m HEAD@{2026-11-26 17:52:16 +1000}: checkout: moving from fix/69-coverage-counts-the-wiring to master
[33m86dd121[m[33m ([m[1;32mfix/69-coverage-counts-the-wiring[m[33m)[m HEAD@{2026-11-26 15:29:25 +1000}: commit (merge): Merge master into the coverage branch
[33m08f78d0[m HEAD@{2026-11-26 15:27:56 +1000}: checkout: moving from fix/68-one-fake to fix/69-coverage-counts-the-wiring
[33m5083d76[m[33m ([m[1;32mfix/68-one-fake[m[33m)[m HEAD@{2026-11-26 14:39:53 +1000}: commit: One fake repository instead of two
[33m8ba3e07[m HEAD@{2026-11-26 14:39:33 +1000}: checkout: moving from master to fix/68-one-fake
[33m8ba3e07[m HEAD@{2026-11-26 14:39:33 +1000}: checkout: moving from fix/66-spec-drift to master
[33mdc82f62[m[33m ([m[1;32mfix/66-spec-drift[m[33m)[m HEAD@{2026-11-26 14:39:19 +1000}: commit: Make the spec and the service agree, or fail
[33m8ba3e07[m HEAD@{2026-11-26 14:38:48 +1000}: checkout: moving from master to fix/66-spec-drift
[33m8ba3e07[m HEAD@{2026-11-26 14:38:48 +1000}: checkout: moving from fix/72-drain-policy to master
[33m31d2b64[m[33m ([m[1;32mfix/72-drain-policy[m[33m)[m HEAD@{2026-11-26 14:33:29 +1000}: checkout: moving from fix/69-coverage-counts-the-wiring to fix/72-drain-policy
[33m08f78d0[m HEAD@{2026-11-26 14:27:52 +1000}: commit: Test the composition root through the database
[33m1478167[m HEAD@{2026-11-26 14:27:52 +1000}: commit: Make the coverage number answer the question it is asked
[33m8ba3e07[m HEAD@{2026-11-26 14:26:58 +1000}: checkout: moving from master to fix/69-coverage-counts-the-wiring
[33m8ba3e07[m HEAD@{2026-11-26 14:26:58 +1000}: checkout: moving from fix/72-drain-policy to master
[33m31d2b64[m[33m ([m[1;32mfix/72-drain-policy[m[33m)[m HEAD@{2026-11-26 14:26:37 +1000}: commit: Report a spent shutdown grace period as a warning, not an error
[33m8ba3e07[m HEAD@{2026-11-26 14:26:12 +1000}: checkout: moving from master to fix/72-drain-policy
[33m8ba3e07[m HEAD@{2026-11-26 14:26:12 +1000}: merge origin/master: Fast-forward
[33m03b3ee2[m HEAD@{2026-11-26 14:26:12 +1000}: checkout: moving from fix/36-server-lifecycle to master
[33mf472675[m[33m ([m[1;32mfix/36-server-lifecycle[m[33m)[m HEAD@{2026-11-26 12:40:30 +1000}: commit: Give the server every timeout, recover panics, and test the lifecycle
[33m03b3ee2[m HEAD@{2026-11-26 12:38:52 +1000}: checkout: moving from master to fix/36-server-lifecycle
[33m03b3ee2[m HEAD@{2026-11-26 12:38:52 +1000}: merge origin/master: Fast-forward
[33m0ec1677[m HEAD@{2026-11-26 12:38:52 +1000}: checkout: moving from fix/75-guard-the-request-path to master
[33m7f00e5d[m[33m ([m[1;32mfix/75-guard-the-request-path[m[33m)[m HEAD@{2026-11-26 11:51:28 +1000}: commit: Guard the request path the generated handler does not guard
[33m5c5dc7c[m[33m ([m[1;32mfeat/58-spec-first-api[m[33m)[m HEAD@{2026-11-26 11:49:47 +1000}: checkout: moving from feat/58-spec-first-api to fix/75-guard-the-request-path
[33m5c5dc7c[m[33m ([m[1;32mfeat/58-spec-first-api[m[33m)[m HEAD@{2026-11-26 11:30:18 +1000}: commit: Cover the validation error message
[33m4fd2ca5[m HEAD@{2026-11-26 11:29:48 +1000}: commit: Treat a malformed snippet id as a missing one in the UI
[33mf0977b3[m HEAD@{2026-11-26 11:28:54 +1000}: commit: Make the spec the source and split the API into tiers
[33m0ec1677[m HEAD@{2026-11-26 11:24:06 +1000}: checkout: moving from master to feat/58-spec-first-api
[33m0ec1677[m HEAD@{2026-11-26 09:41:50 +1000}: checkout: moving from master to master
[33m0ec1677[m HEAD@{2026-11-26 09:37:06 +1000}: pull: Fast-forward
[33m3e3523b[m HEAD@{2026-11-26 09:37:00 +1000}: checkout: moving from feat/28-dependabot to master
[33mff34ec4[m[33m ([m[1;32mfeat/28-dependabot[m[33m)[m HEAD@{2026-11-26 09:30:46 +1000}: commit: add dependabot for routine version updates
[33mc89917f[m[33m ([m[1;32mfeat/26-image-vulns[m[33m)[m HEAD@{2026-11-26 09:30:39 +1000}: checkout: moving from feat/26-image-vulns to feat/28-dependabot
[33mc89917f[m[33m ([m[1;32mfeat/26-image-vulns[m[33m)[m HEAD@{2026-11-26 09:22:26 +1000}: commit: limit the sarif severities so the gate matches --severity
[33maad6273[m HEAD@{2026-11-26 09:20:18 +1000}: commit: scan the images in ci and make high/critical unshippable
[33m828261e[m[33m ([m[1;32mfeat/24-deps-targets[m[33m)[m HEAD@{2026-11-26 09:15:13 +1000}: checkout: moving from feat/24-deps-targets to feat/26-image-vulns
[33m828261e[m[33m ([m[1;32mfeat/24-deps-targets[m[33m)[m HEAD@{2026-11-26 09:09:11 +1000}: commit: add make targets for what actually ships: deps, deps-image, deps-build, deps-graph, why, symbols
[33m5da15a9[m[33m ([m[1;32mfeat/22-vuln-target[m[33m)[m HEAD@{2026-11-26 09:08:39 +1000}: checkout: moving from feat/22-vuln-target to feat/24-deps-targets
[33m5da15a9[m[33m ([m[1;32mfeat/22-vuln-target[m[33m)[m HEAD@{2026-11-26 09:05:29 +1000}: commit: add make vuln and make vuln-binary (pinned govulncheck)
[33m39745ad[m[33m ([m[1;32mfeat/20-restful-routes[m[33m)[m HEAD@{2026-11-26 09:05:11 +1000}: checkout: moving from feat/20-restful-routes to feat/22-vuln-target
[33m39745ad[m[33m ([m[1;32mfeat/20-restful-routes[m[33m)[m HEAD@{2026-11-26 08:56:57 +1000}: commit: use the resource shape for the api: /api/snippet and /api/snippet/{id}
[33m3e3523b[m HEAD@{2026-11-26 08:56:01 +1000}: checkout: moving from master to feat/20-restful-routes
[33m3e3523b[m HEAD@{2026-11-26 08:56:01 +1000}: pull --ff-only -q: Fast-forward
[33m7a6903e[m HEAD@{2026-11-26 08:55:58 +1000}: checkout: moving from feat/15-readme to master
[33m3563a6d[m[33m ([m[1;32mfeat/15-readme[m[33m)[m HEAD@{2026-11-26 08:49:50 +1000}: commit: add a README, an MIT licence, and drop the empty internal/ directory
[33m7a6903e[m HEAD@{2026-11-26 08:49:29 +1000}: checkout: moving from master to feat/15-readme
[33m7a6903e[m HEAD@{2026-11-26 08:49:27 +1000}: pull --ff-only -q: Fast-forward
[33m91bfdd0[m HEAD@{2026-11-26 08:49:24 +1000}: checkout: moving from feat/17-delete-snippet to master
[33mbd933a8[m[33m ([m[1;32mfeat/17-delete-snippet[m[33m)[m HEAD@{2026-11-26 08:45:08 +1000}: commit: add DELETE for snippets and the e2e lifecycle it unblocks
[33m43614e0[m[33m ([m[1;32mfeat/13-create-snippet[m[33m)[m HEAD@{2026-11-26 08:44:31 +1000}: checkout: moving from feat/13-create-snippet to feat/17-delete-snippet
[33m43614e0[m[33m ([m[1;32mfeat/13-create-snippet[m[33m)[m HEAD@{2026-11-26 08:32:40 +1000}: commit: make POST /api/snippet/create insert, and add the form
[33m91bfdd0[m HEAD@{2026-11-26 08:31:44 +1000}: checkout: moving from master to feat/13-create-snippet
[33m91bfdd0[m HEAD@{2026-11-26 08:28:23 +1000}: checkout: moving from test/gate-blocks-broken-lb to master
[33ma95bffc[m HEAD@{2026-11-26 08:26:55 +1000}: commit: DO NOT MERGE: misroute /api so the e2e gate has something to catch
[33m91bfdd0[m HEAD@{2026-11-26 08:26:55 +1000}: checkout: moving from master to test/gate-blocks-broken-lb
[33m91bfdd0[m HEAD@{2026-11-26 08:26:40 +1000}: pull --ff-only -q: Fast-forward
[33meb1ebd8[m HEAD@{2026-11-26 08:26:36 +1000}: checkout: moving from master to master
[33meb1ebd8[m HEAD@{2026-11-26 08:25:42 +1000}: pull --ff-only -q: Fast-forward
[33mb8e2364[m HEAD@{2026-11-26 08:25:39 +1000}: checkout: moving from feat/4-e2e to master
[33m1179449[m[33m ([m[1;32mfeat/4-e2e[m[33m)[m HEAD@{2026-11-26 08:23:53 +1000}: checkout: moving from feat/4-e2e to feat/4-e2e
[33m1179449[m[33m ([m[1;32mfeat/4-e2e[m[33m)[m HEAD@{2026-11-26 08:23:53 +1000}: branch: Reset to origin/feat/3-ui-client
[33m511bd03[m HEAD@{2026-11-26 08:20:16 +1000}: checkout: moving from feat/3-ui-client to feat/4-e2e
[33m511bd03[m HEAD@{2026-11-26 08:19:58 +1000}: commit: add an e2e check that runs in ci, not just on a laptop
[33m449d2cd[m HEAD@{2026-11-26 08:10:35 +1000}: commit: ui: fetch the api, add list and view routes
[33mb8e2364[m HEAD@{2026-11-26 08:09:33 +1000}: checkout: moving from master to feat/3-ui-client
[33mb8e2364[m HEAD@{2026-11-26 08:05:44 +1000}: checkout: moving from fix/ci-node24 to master
[33m4685f00[m HEAD@{2026-11-26 08:04:13 +1000}: rebase (finish): returning to refs/heads/fix/ci-node24
[33m4685f00[m HEAD@{2026-11-26 08:04:13 +1000}: rebase (pick): bump the actions to the majors that run on node24
[33mb8e2364[m HEAD@{2026-11-26 08:04:13 +1000}: rebase (start): checkout origin/master
[33ma27d73b[m HEAD@{2026-11-26 08:04:13 +1000}: checkout: moving from master to fix/ci-node24
[33mb8e2364[m HEAD@{2026-11-26 08:04:13 +1000}: pull --ff-only -q: Fast-forward
[33meecd585[m HEAD@{2026-11-26 08:04:10 +1000}: checkout: moving from fix/ci-node24 to master
[33ma27d73b[m HEAD@{2026-11-26 08:03:12 +1000}: commit: bump the actions to the majors that run on node24
[33ma1451e4[m HEAD@{2026-11-26 08:03:12 +1000}: checkout: moving from feat/api-json to fix/ci-node24
[33mb7eee28[m HEAD@{2026-11-26 07:54:50 +1000}: commit: return json from the api and wire snippetsAll/serverError
[33ma1451e4[m HEAD@{2026-11-26 07:54:15 +1000}: checkout: moving from feat/split-containers to feat/api-json
[33ma1451e4[m HEAD@{2026-11-26 07:53:06 +1000}: commit: add ci: vet, build, migrate and test against a postgres service
