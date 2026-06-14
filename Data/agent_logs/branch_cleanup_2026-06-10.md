# Branch & Remote Cleanup — 2026-06-10

Post-pivot housekeeping. GitLab→GitHub migration achieved; GitHub (`origin`) is sole source.
Recovery: `git branch <name> <sha>` restores any deleted branch; `git remote add <name> <url>`
restores any removed remote. Reflog also retains tips ~90 days.

## Branches deleted (merged into dev — safe)
| Branch | Tip SHA |
|---|---|
| feature/boardroom | b4d18c91e534620e572707928a9b4e8d17677061 |
| feature/go-client-standalone | 3540dc70f2a645bb29755d6c3429dc58a56a02fa |
| features/SPLIT-001-monorepo-restructure | 1ea94e49b0287a304f9002ac662de195af856f52 |
| fix/admin-health-check-deadlock | 97f72a2bb224006e5ecc48d04e0108307ea6317c |
| fix/ci-remaining-test-failures | 0625eede47ae33986d55f15e1bbef3894cd7e7c4 |
| fix/ci-sqlalchemy-test-api | 60b84e04d1765a0cb040bda65892c5fe4fb3210b |
| fix/windows-installer-exe-name | 9e942e022259ab5b072e5c192f7208f584a7dc31 |

## Branches force-deleted (shipped via squash / migration achieved)
| Branch | Tip SHA |
|---|---|
| chore/TASK-041-migrate-wiki | 73d9eee0419f5dad9c769d5a790e20c70ae3152d |
| feat/TASK-040-logo-and-windows-icon | 849978db4f9db67a1a9e1bc5ecca026490f0918a |
| feat/client-server-decoupling | c65111e01dd09e7cbb08596bb5ae8ffd6441e7b7 |
| feat/go-native-git-commit-flow | b820f255e563df34c125d85ddfa8caa496c447a5 |
| features/TASK-020-windows-force-trigger | 46fd096803a01e0b5f07324f2f4cbc48bab5ca12 |
| features/TASK-021-windows-sighup-reload | fd799966dac9609ce28115a2f1b04e36b2596079 |
| fix/TASK-025-windows-native-build | 5bf76666ee802cd94e7f61ec0edd087d0fcdb8cb |
| migration | 46fd096803a01e0b5f07324f2f4cbc48bab5ca12 (== TASK-020 tip) |

## Remotes removed (vestigial GitLab — no automated push references found)
| Remote | URL |
|---|---|
| gitlab-client | git@gitlab.com:devtrack3_cloud/devtrack_client.git |
| gitlab-server | git@gitlab.com:devtrack3_cloud/devtrack_server.git |
| gitlab-wiki | git@gitlab.com:devtrack3_cloud/devtrack_wiki.git |

## Origin (GitHub) branches deleted — 2026-06-10
Recover any with: git push origin <sha>:refs/heads/<name>
| Branch | SHA |
|---|---|
| bot_automation | 9034cf984b16cf603360fc944e4ad5d34695fd57 |
| chore/TASK-041-migrate-wiki | 73d9eee0419f5dad9c769d5a790e20c70ae3152d |
| feat/TASK-040-logo-and-windows-icon | 849978db4f9db67a1a9e1bc5ecca026490f0918a |
| feat/client-server-decoupling | c65111e01dd09e7cbb08596bb5ae8ffd6441e7b7 |
| feat/go-native-git-commit-flow | b820f255e563df34c125d85ddfa8caa496c447a5 |
| feature/go-client-standalone | 117bd588446b7bd171d70675cffed854d3d4183b |
| feature/saas_model | 754a2a3533a515eeff5d9164165322305a98d387 |
| features/SPLIT-001-monorepo-restructure | 1ea94e49b0287a304f9002ac662de195af856f52 |
| features/TASK-009-server-tui-tests | 665370bd020ac15f7d213c60c8f1109569dd2154 |
| features/TASK-009-ticket-cache | 2eff33a03062d772d2a4dfaee2e349870a11b70b |
| features/TASK-011-admin-route-tests | 12d268e8e1c1d68e544d7257730c9095dd4b4704 |
| features/TASK-012-user-role-disable | 2ef7f144c1558213e0b5495bec80287c879ea992 |
| features/TASK-013-license-page | a221c04ee057ef43ef662e3ca0590b3ba0356332 |
| features/TASK-014-trigger-stats-panel | 5337f2f33ef8a963ade73f0cbedee9f022b58135 |
| features/TASK-015-cs3-polish | e740c99de35f10c1a4d458ac40399d4908d2aa92 |
| features/TASK-018-windows-build-tags | 2c55fac3d2b3655b0d11bde6a862a03d76efdb66 |
| features/client_server_arch | 8006be5744466b5d8e125cd929e84da4b5fdcf7b |
| features/loadEnvs | e25ca139c7b2129c8a11bc3144be9e800a5b2e10 |
| features/release_ready | 54342cb282eceeb31fef9e5aac3e6e8535e46e67 |
| features/standalone-cli-mode | 65c353416596df1d1cf3cfb9558e0a6e8c28b42c |
| fix/TASK-007-remaining-getenv | df59693200e2dae2303aa7e8afec70b1db2165b5 |
| fix/TASK-016-auth-hardcoded-values | 25cec2f01a1968551b6d0c9a5809abaa35f79973 |
| fix/TASK-017-medium-hardcoded-values | 46f2cda850448874630ff869c195a3296b4a7565 |
| fix/TASK-025-windows-native-build | e0c45b960620764969083dcc88f9d1af9c5b3ca6 |
| fix/admin-health-check-deadlock | 97f72a2bb224006e5ecc48d04e0108307ea6317c |
| fix/ci-remaining-test-failures | 0625eede47ae33986d55f15e1bbef3894cd7e7c4 |
| fix/ci-sqlalchemy-test-api | 60b84e04d1765a0cb040bda65892c5fe4fb3210b |
| migration | 46fd096803a01e0b5f07324f2f4cbc48bab5ca12 |
| sage-improvements | e7d5e9985915e7d276d95e0abca71ead978e3db3 |
