## [unreleased]

### 🚀 Features

- Migrate from GitLab to GitHub ([73ea246](https://github.com/Ghent/capacitarr/commit/73ea246a22e83c48a7f7dadc429b73079392b608))
- Migrate from GitLab to GitHub ([e390302](https://github.com/Ghent/capacitarr/commit/e39030251ed7e903b4ec8ff0be30cc3df3b6ecd4))

### 🐛 Bug Fixes

- *(lint)* Remove invalid golangci-lint v2 config fields ([dd107bc](https://github.com/Ghent/capacitarr/commit/dd107bca352160c3086165c843eaa1f804dd33ef))
- *(lint)* Add config verify to all golangci-lint Makefile targets ([eda7ea0](https://github.com/Ghent/capacitarr/commit/eda7ea062a7a605beb86b9b786ec5d4733a19f21))
- *(ci)* Use correct trivy-action version tag ([eac53db](https://github.com/Ghent/capacitarr/commit/eac53dbf27252cf0f9800c64878b95d45fad6bf7))
- *(site)* Handle pnpm install in Cloudflare build environment ([cf66833](https://github.com/Ghent/capacitarr/commit/cf668331c20762485bb90ca667e388f1ae359e85))
- *(site)* Fix pnpm detection in Cloudflare build ([6985449](https://github.com/Ghent/capacitarr/commit/69854497cd19c223cd60b058e96819fdc078ce23))
- *(site)* Use Cloudflare's built-in pnpm instead of corepack ([d69f42c](https://github.com/Ghent/capacitarr/commit/d69f42c65973a69c5a2d227da3d605021badfd87))
- *(site)* Self-install pnpm when not available in build env ([68cc05c](https://github.com/Ghent/capacitarr/commit/68cc05c9e1cb887fba49bf9fe41d05621a89ffc4))
- *(site)* Simplify build script — install pnpm unconditionally ([2eb7481](https://github.com/Ghent/capacitarr/commit/2eb74818bf2bbbb6d5c02ce001f79106d1502068))
- *(site)* Update remaining GitLab references and fix link rewriting ([7efefdf](https://github.com/Ghent/capacitarr/commit/7efefdfde0e7a8949f241e77868db316972ab9ec))
- *(site)* Correct Quick Start link path on landing page ([31dc9ae](https://github.com/Ghent/capacitarr/commit/31dc9ae00e84a8b811ed1ad05bfd24e267e33daa))
- *(site)* Use neutral color for GitHub icon in RepoStats ([b0b6b41](https://github.com/Ghent/capacitarr/commit/b0b6b41837ec4e91e96c219d3a3f0da525223445))
## [2.2.1] - 2026-03-26

### 🐛 Bug Fixes

- *(changelog)* Reclassify feat(docs) commits as docs in git-cliff ([3815406](https://github.com/Ghent/capacitarr/commit/38154068fff9b73c7b6acf21671e53ef2d9c5418))
- *(integrations)* Use correct v-model binding for modal switches ([2b14373](https://github.com/Ghent/capacitarr/commit/2b14373b56bd02d140c63a723b53d5b9fbc5bee3)) — reported by @tomislavf ([#8](https://github.com/Ghent/capacitarr/issues/8))
## [2.2.0] - 2026-03-26

### 🚀 Features

- *(collections)* Multi-source collection deletion with approval grouping ([7cae95f](https://github.com/Ghent/capacitarr/commit/7cae95feb9550a1df99707ea3d651f4392a84f4c))
- *(integrations)* Add Tracearr integration support ([a4a3e43](https://github.com/Ghent/capacitarr/commit/a4a3e43607305ea6a7e9b7f258ef2c3ba3c089cc)) — reported by @tomislavf ([#10](https://github.com/Ghent/capacitarr/issues/10))

### 🐛 Bug Fixes

- *(library)* Make Shows filter display seasons grouped by show ([4ad6cae](https://github.com/Ghent/capacitarr/commit/4ad6cae0638e2e60ba3f5167ae7297742e5c995c)) — reported by @tomislavf ([#9](https://github.com/Ghent/capacitarr/issues/9))
- *(library)* Improve shows/seasons UX across library management ([71a779c](https://github.com/Ghent/capacitarr/commit/71a779c60e0a11531191bb317efac749fdf6c7f0)) — reported by @tomislavf ([#9](https://github.com/Ghent/capacitarr/issues/9))
- *(ui)* Misc UI fixes — filters, selection, score colors, deletion priority ([9915978](https://github.com/Ghent/capacitarr/commit/99159786b98cbf4a5e41a91018e086bc0f90ab79)) — reported by @tomislavf ([#9](https://github.com/Ghent/capacitarr/issues/9))
- *(ui)* Resolve poster dimming in deletion priority grid view ([961a880](https://github.com/Ghent/capacitarr/commit/961a88013bf00efa9a858a1a16bf4ef7dc9c68d4)) — reported by @tomislavf ([#10](https://github.com/Ghent/capacitarr/issues/10))
- *(library)* Filter media type buttons by configured integrations ([169959a](https://github.com/Ghent/capacitarr/commit/169959af4a0dcd008171172bdc3a6602392b1422)) — reported by @tomislavf ([#9](https://github.com/Ghent/capacitarr/issues/9))
- *(engine)* Exclude scoring factors when integrations are broken ([e757024](https://github.com/Ghent/capacitarr/commit/e757024a61d3b8f8cbe729ed1fc376689a4f882e))
- *(engine)* Only count enricher errors toward capability failure ([ae71000](https://github.com/Ghent/capacitarr/commit/ae71000f39bdcf8bbee6e21edb6344a3e7c54b01))
- *(engine)* Remove Layer 2 from RequestPopularityFactor ([f7573c9](https://github.com/Ghent/capacitarr/commit/f7573c949790674f2b3de6ec49de5ae7cf6c839d))
- *(seerr)* Fix connection test and request count aggregation ([cb4b4c9](https://github.com/Ghent/capacitarr/commit/cb4b4c9726f31419f7e90d9568a3b732049ed4f5))
- *(integrations)* Add collection autocomplete for Jellyfin and Emby ([e626b75](https://github.com/Ghent/capacitarr/commit/e626b7589c96a2b41d73c3ed80ea3337e21292fd)) — reported by @tomislavf
- *(tracearr)* Use correct Public API endpoints verified from source ([b4bf49f](https://github.com/Ghent/capacitarr/commit/b4bf49f272907ec264d30bcbe288ae5b53a8f241)) — reported by @tomislavf

### 🛡️ Security

- Full codebase audit — security, docs, and code quality ([5701438](https://github.com/Ghent/capacitarr/commit/5701438a829b55d669fcb4881e4b1478a393adff))
## [2.1.0] - 2026-03-24

### 🚀 Features

- *(sonarr)* Add show-level-only evaluation toggle ([1949a90](https://github.com/Ghent/capacitarr/commit/1949a90b9a69917b9f307d5e5fafa42d2fa0c06c)) — reported by @tomislavf ([#8](https://github.com/Ghent/capacitarr/issues/8))

### 🐛 Bug Fixes

- *(deletion)* Clear queue on execution mode change ([bfa5a21](https://github.com/Ghent/capacitarr/commit/bfa5a2186a2efa304a847902b342d3b69eb98805))
- *(engine)* Exclude inapplicable scoring factors from evaluation ([420dfc9](https://github.com/Ghent/capacitarr/commit/420dfc98569dc40128b39e832ea95bd115889f57)) — reported by @tomislavf ([#6](https://github.com/Ghent/capacitarr/issues/6)) ([#7](https://github.com/Ghent/capacitarr/issues/7))
- *(jellystat)* Use x-api-token header instead of Authorization Bearer ([42d9731](https://github.com/Ghent/capacitarr/commit/42d9731deab4df67b9f06285ef1824aa0ebf6583)) — reported by @tomislavf ([#5](https://github.com/Ghent/capacitarr/issues/5))

### 🛡️ Security

- *(deps)* Pin all Docker images and eliminate curl-pipe-to-shell ([04ab628](https://github.com/Ghent/capacitarr/commit/04ab6287fbd1d4dd9ad4e8b31239133193dc52f0))
- Comprehensive codebase audit ([d07346f](https://github.com/Ghent/capacitarr/commit/d07346fac23468faf5edef55e8891d21ba3561e9))
## [2.0.0] - 2026-03-24

### 🚀 Features

- *(db)* [**breaking**] Replace 1.x incremental migrations with 2.0 baseline schema ([dbd9e3b](https://github.com/Ghent/capacitarr/commit/dbd9e3bc452cbd3b381dd11ed9329b10fc27ce40))
- *(integrations)* Define capability interfaces for 2.0 ([c4af106](https://github.com/Ghent/capacitarr/commit/c4af106bcd721dba0f2fd2135b3f6b95588a5a63))
- *(integrations)* Add IntegrationRegistry with capability-based discovery ([f78e2e5](https://github.com/Ghent/capacitarr/commit/f78e2e538e09d46e602363ed2e5755a7ef7e0b3c))
- *(services)* Add LibraryService with threshold resolution ([6f75afe](https://github.com/Ghent/capacitarr/commit/6f75afe5bd0b0649a8dc6e65c89c1fcf74a5d5a1))
- *(engine)* Add pluggable scoring factors with ScoringFactor interface ([93a5824](https://github.com/Ghent/capacitarr/commit/93a582434dcc63ec736320a61d8c988db3417850))
- *(integrations)* Add pluggable enrichment pipeline ([9ea631e](https://github.com/Ghent/capacitarr/commit/9ea631e597f4939396149c75c2cda068271fd210))
- *(engine)* Extract reusable Evaluator from poller ([1204bfc](https://github.com/Ghent/capacitarr/commit/1204bfc4d73048e2a45d6e56d502e58905e8bdc8))
- *(integrations)* Implement WatchlistProvider and RequestProvider on clients ([904b408](https://github.com/Ghent/capacitarr/commit/904b408a8008a8f599a5dc72c9355b65ab471087))
- *(services)* Add AnalyticsService and WatchAnalyticsService with API routes ([e328622](https://github.com/Ghent/capacitarr/commit/e3286225d63edc87121aee1aa98f7873a25d7c4d))
- *(routes)* Add Library CRUD API endpoints ([d2da99d](https://github.com/Ghent/capacitarr/commit/d2da99db359e35b2de9460d8663e17578e111a10))
- *(integrations)* Add plugin-style factory registration ([6bb490e](https://github.com/Ghent/capacitarr/commit/6bb490e13c6357711d3eeee045624de068dd8717))
- *(services)* Add BuildIntegrationRegistry using factory+capability pattern ([35bac22](https://github.com/Ghent/capacitarr/commit/35bac22dbf973ae315fb4f96b45f299c9962f9b0))
- *(rules)* Add rule impact preview with API endpoint ([83417a1](https://github.com/Ghent/capacitarr/commit/83417a154135ca4f695a32ef304d29ad2a11d3bb))
- *(frontend)* Replace ApexCharts with ECharts (Phase 3 start) ([8900e4e](https://github.com/Ghent/capacitarr/commit/8900e4ed22612e960400248825aacca8fa30efb1))
- *(frontend)* Add DashboardCard component for analytics pages ([f8e0b6e](https://github.com/Ghent/capacitarr/commit/f8e0b6e930d7af570802643667d591f6be6982ce))
- *(frontend)* Add Insights page with three-tab layout (Phase 4) ([dbed697](https://github.com/Ghent/capacitarr/commit/dbed697b5c660445094776cc4545732ba72257dc))
- *(frontend)* Add Insights nav link and i18n keys ([c64cff2](https://github.com/Ghent/capacitarr/commit/c64cff25b3331d62c8fa51a4282b9dfa31b1ca40))
- *(migration)* Add 1.x → 2.0 one-way data import (Phase 7) ([cee90b7](https://github.com/Ghent/capacitarr/commit/cee90b709fcf2ef57b9286668c7afeeda976567a))
- *(ui)* Add CreatableCombobox component with create-on-type ([1b78b18](https://github.com/Ghent/capacitarr/commit/1b78b187b21e2b4d554666b1e0cc0148e3cf1d13))
- *(frontend)* Add virtual scrolling to dashboard activity feed ([c169605](https://github.com/Ghent/capacitarr/commit/c169605108633b6d21eb2593a8bc1b1f49714fcf))
- *(events)* Add analytics_updated SSE event on preview cache refresh ([a2dd0cf](https://github.com/Ghent/capacitarr/commit/a2dd0cfb6e98b742f8e0b2efc7ff1c76027937d7))
- *(frontend)* Complete Phase 4 frontend restructuring ([7decba0](https://github.com/Ghent/capacitarr/commit/7decba08222292455f180404710ea1c518d74f5a))
- *(frontend)* Wire Phase 6 frontend UI to backend APIs ([db534e1](https://github.com/Ghent/capacitarr/commit/db534e156ba4790a898748a867bdf11da5b17da1))
- *(migration)* Wire Phase 7 migration CLI, API, and frontend ([234c6b4](https://github.com/Ghent/capacitarr/commit/234c6b427989e9fc8aaf09ded94d4502d5375d4b))
- *(dashboard)* Add score column, startup poll, deletion queue SSE, chart upgrades ([c7bbda2](https://github.com/Ghent/capacitarr/commit/c7bbda2e5f4dc60cffb24408e945bb805cc0f373))
- *(insights)* Redesign insights page with capacity-focused visualizations ([05c25b8](https://github.com/Ghent/capacitarr/commit/05c25b8fc1cdc7834eea8f49ce899f24c68e9f90))
- *(approval)* Add per-cycle queue reconciliation and threshold-triggered engine runs ([35617bc](https://github.com/Ghent/capacitarr/commit/35617bc648e46a8efb60313eb27ecb1abf4b04ad))
- *(deletion)* Add grace period, snooze, and clear queue (Phase 3) ([6822971](https://github.com/Ghent/capacitarr/commit/682297115e0f8b174349ca05265f7e21ba1ef313))
- *(ui)* Always-visible deletion queue card with mode-specific empty states ([0ce2e72](https://github.com/Ghent/capacitarr/commit/0ce2e72cc1571fe3362037f79604cdc2323fd09e))
- *(deletion)* Return dry-deleted approval items to pending status ([771ee12](https://github.com/Ghent/capacitarr/commit/771ee12d6e6bf21a164a479312fe0aed40d7f5e8))
- *(rules)* Add combined rule context endpoint and extract field definitions ([5504677](https://github.com/Ghent/capacitarr/commit/5504677e8b7e58a11f203f2ab564255ceef7d44b))
- *(rules)* Add edit custom rules UI with card state-swap pattern ([c693a9a](https://github.com/Ghent/capacitarr/commit/c693a9ac1d1eb5bd65dd27411b3bed599128925d))
- *(ui)* V-motion presets, virtual scrolling, disk group sparkline ([0076e13](https://github.com/Ghent/capacitarr/commit/0076e13645c9c9d1dabc14d4384e25b324227b19))
- *(ui)* Replace disk group bar+sparkline with gauge arc ([f67f70e](https://github.com/Ghent/capacitarr/commit/f67f70e251f939430c3a103bbce41c5e3b1d8739))
- *(ui)* Add target/threshold pointer markers on gauge arc ([7870189](https://github.com/Ghent/capacitarr/commit/787018922d7f0ff6b5f7f441c76a7d322734c182))
- *(ui)* Gauge pulse fix, responsive disk group grid ([730ee1b](https://github.com/Ghent/capacitarr/commit/730ee1bbe29bbeced825e9399d9574ad5682b23d))
- *(integrations)* Add Jellystat integration for Jellyfin analytics ([fb1c12f](https://github.com/Ghent/capacitarr/commit/fb1c12fa1c69456691f93fc3787f414fc0332094))
- *(enrichment)* Add enrichment observability and match rate logging ([647c779](https://github.com/Ghent/capacitarr/commit/647c779e7fb44ea64136f5c295b14c5f72940c23))
- *(backup)* Rework import modes to merge/sync with per-item sync ([204d9e4](https://github.com/Ghent/capacitarr/commit/204d9e47810cad09c3de628cd3f60792dd84e97a))
- *(backup)* Add full-section import preview with field-level diffs ([92f81f6](https://github.com/Ghent/capacitarr/commit/92f81f69d0b52414eca6884c730fe7c45bd4ea5b))
- *(backup)* Add stepper import flow with inline diff view ([ee30ad6](https://github.com/Ghent/capacitarr/commit/ee30ad6d4ee5d726db4069b4947c05db65318043))
- *(collections)* Add collection deletion data model and Radarr resolver ([750def3](https://github.com/Ghent/capacitarr/commit/750def35c7a2781a7b53c1d49d4a62f93ed62c18))
- *(collections)* Add collection expansion in poller and deletion pipeline ([ad89d2a](https://github.com/Ghent/capacitarr/commit/ad89d2ae3b7b2850f1d1b72713805061baf64a48))
- *(collections)* Add collection enrichment from Plex, Jellyfin, and Emby ([8340d05](https://github.com/Ghent/capacitarr/commit/8340d05baf94ff7ad01bf238a76437f3af75ce53))
- *(collections)* Add collection indicators to frontend components ([78ef10f](https://github.com/Ghent/capacitarr/commit/78ef10f844763ae809061f13806a952dfe201541))
- *(collections)* Add integration settings toggle for collection deletion ([dc5ba91](https://github.com/Ghent/capacitarr/commit/dc5ba91d007bf7f135d4f3d5f63e2c282ad4a00d))
- *(collections)* Add collection context to notification digest ([856a5c4](https://github.com/Ghent/capacitarr/commit/856a5c4a0d89e10e0318f421efa88e8fa6d9a27a))
- *(migration)* Trigger engine run after 1.x → 2.0 migration ([e957ffc](https://github.com/Ghent/capacitarr/commit/e957ffcfe9b98bb8ab34229e8b42cde016f65307))
- *(dashboard)* Reorder queue cards — deletion first ([a6ccfd6](https://github.com/Ghent/capacitarr/commit/a6ccfd66c7456cefa8670aa46d970eb6cc56d1ca))
- Mode-aware sparkline with ghost series, evaluated band, pulse ([d94076c](https://github.com/Ghent/capacitarr/commit/d94076c3fb92c09b2cce20006f1f3b833276084a))

### 🐛 Bug Fixes

- *(frontend)* Complete overseerr to seerr rename in frontend ([2e10f85](https://github.com/Ghent/capacitarr/commit/2e10f8562c91fb901e44d8a0a0e474cd20d68d3d))
- *(migration)* Redesign 1.x → 2.0 migration workflow ([43c3a1e](https://github.com/Ghent/capacitarr/commit/43c3a1e4b1d61aa7e0bb5ec0ed26bef4e5a685ba))
- *(insights)* Resolve chart color parsing and remove unusable charts ([729a7e8](https://github.com/Ghent/capacitarr/commit/729a7e82cde2b121eb0c8171abd18f6cea793503))
- *(preview)* Persist media cache to database for restart recovery ([5240db4](https://github.com/Ghent/capacitarr/commit/5240db4b57380dd5c5dfc8eb79c56b73799917c5))
- *(poller)* Fix approval queue population and ClearQueue cross-contamination ([10cb4fb](https://github.com/Ghent/capacitarr/commit/10cb4fb7fe7959dba1dd07ee523483e7dcc7c7f0))
- *(poller)* Use EventBus for run triggers and reset timer on settings change ([1ec2582](https://github.com/Ghent/capacitarr/commit/1ec25820094be6f7328881725830210787d382e1))
- *(db)* Merge media_cache migration into v2 baseline ([54b7330](https://github.com/Ghent/capacitarr/commit/54b7330680cee38ce128c2efb714b4e4f73772ce))
- *(rules)* Extract shared validation and fix Update() missing validation ([92358a9](https://github.com/Ghent/capacitarr/commit/92358a93541797d4a29da9c77052704457a199dc))
- Resolve goconst and prettier lint issues ([b85226d](https://github.com/Ghent/capacitarr/commit/b85226dfe18536de89f047a97a70830bef8c5db9))
- *(rules)* Remove global rules concept and add radio group component ([36956f6](https://github.com/Ghent/capacitarr/commit/36956f6081f4f14976fee4f6dffec6b76664e5db))
- *(integrations)* Fix error display, add enable toggle, remove unfinished features ([529acfa](https://github.com/Ghent/capacitarr/commit/529acfa349e623fd6100ec238b1d272d05acfded))
- *(ui)* Adaptive sparkline y-axis, label collision, date range sync ([6acdebe](https://github.com/Ghent/capacitarr/commit/6acdebe04ed55320331e8e753a100f1af883856d))
- *(ui)* Register GaugeChart in ECharts plugin ([efb9eea](https://github.com/Ghent/capacitarr/commit/efb9eead840ff041137d31677334672f29befa20))
- *(ui)* Replace speedometer pointers with small triangle carets ([ccd08e7](https://github.com/Ghent/capacitarr/commit/ccd08e7ea7824f95f8632ba5ce51c764d8408d11))
- *(ui)* Replace speedometer pointers with subtle arc nubs ([0d72a24](https://github.com/Ghent/capacitarr/commit/0d72a24149e7d881ee7601bf99d7ed2d42f3c50c))
- *(ui)* Space threshold markers on inner/outer edges of gauge arc ([7c46460](https://github.com/Ghent/capacitarr/commit/7c464604afbd8457b40de5854076a4ebad5afa0a))
- *(ui)* Push outer threshold triangle further from arc edge ([abe5849](https://github.com/Ghent/capacitarr/commit/abe5849d1953fc2d914ec758f73a683afc399881))
- *(ui)* Remove threshold triangle markers from gauge ([75fb0d9](https://github.com/Ghent/capacitarr/commit/75fb0d9dc89a40bae8db80d6a2a61fb0ccfd9610))
- *(ui)* Use rgba for gauge pulse animation instead of oklch var ([69a1c9c](https://github.com/Ghent/capacitarr/commit/69a1c9c23f1bfbfd1d211b9e842823fbc56073b4))
- *(enrichment)* Aggregate watch data across all Jellyfin/Emby users and match by TMDb ID ([e3fb52a](https://github.com/Ghent/capacitarr/commit/e3fb52a8d810663f94e465833578d181e9993b02)) — reported by @Thundernerd ([#3](https://github.com/Ghent/capacitarr/issues/3))
- *(enrichment)* Wire Tautulli enricher via TMDb→RatingKey map ([1da5b75](https://github.com/Ghent/capacitarr/commit/1da5b7572e0775aca59267838e671d5c5c0ae8d1))
- *(db)* Add missing TableName() for MediaCache singleton ([0dec246](https://github.com/Ghent/capacitarr/commit/0dec246d81e538c484768be7386f30df08f5ba86))
- *(rules)* Fix CreatableCombobox dropdown in edit mode ([cfb197f](https://github.com/Ghent/capacitarr/commit/cfb197f4456ca5a55d30d9c7b1d975c1bcfa4670))
- *(rules)* Preserve enabled state when editing a rule ([01980a7](https://github.com/Ghent/capacitarr/commit/01980a7d373a9707331d99725b092a1b0ae8af43))
- *(engine)* Show run completion time instead of start time ([af4d915](https://github.com/Ghent/capacitarr/commit/af4d9152e4745b9d09130acbc0a6f78c3bdd5124))
- *(integrations)* Resolve poster URLs against *arr base URL ([42807f9](https://github.com/Ghent/capacitarr/commit/42807f900c3d559dc53cbc6acfecd91af98cddd6))
- *(approval)* Await engineFetchStats before fetching approval queue ([439910d](https://github.com/Ghent/capacitarr/commit/439910d468085c6de9fec408e3b38bc3ec0b9faf))
- *(enrichment)* Aggregate episode watch data into parent series for Jellyfin and Emby ([9bf19f7](https://github.com/Ghent/capacitarr/commit/9bf19f73cb1e5b191a514a2b6dc60be6b49b851b)) — reported by @Thundernerd ([#4](https://github.com/Ghent/capacitarr/issues/4))
- *(migration)* Persist 1.x scoring weights to scoring_factor_weights table ([207670d](https://github.com/Ghent/capacitarr/commit/207670dd131f5dd87ea4d2acac86bd41ca8d63a2))
- *(ci)* Resolve noctx lint violations and Makefile CI parity ([cd64073](https://github.com/Ghent/capacitarr/commit/cd640732fe33a24ac5b9c1bacb1d8217a66110e6))
- *(site)* Correct broken dashboard image reference ([cad7a50](https://github.com/Ghent/capacitarr/commit/cad7a50960f5b36f3ad71a4a2a8196e62bc94a55))
- *(ui)* Use invisible pseudo-element for switch touch targets ([9b73a6c](https://github.com/Ghent/capacitarr/commit/9b73a6c9b9cf5c351b39193085f3fa6811f10f64))
- *(frontend)* Resolve browser console errors and warnings ([febdad8](https://github.com/Ghent/capacitarr/commit/febdad85c29c66b153ad0fca56450996c27c7ef7))
- *(migration)* Overhaul 1.x→2.0 migration safety and correctness ([acda87b](https://github.com/Ghent/capacitarr/commit/acda87b4b8ae603a05e704dbaacaaefd58c73d09))
- *(dashboard)* Move axisPointer cross config to tooltip ([e937dc6](https://github.com/Ghent/capacitarr/commit/e937dc639df84a49cd0ead1ee96749003ffc65ed))
- Add post-migration schema fixup for existing databases ([416c046](https://github.com/Ghent/capacitarr/commit/416c046ba09787f84ecb4257c864aac4b4e4e421))
- *(dashboard)* Replace auto-refresh timer with SSE-driven updates ([83080a6](https://github.com/Ghent/capacitarr/commit/83080a62426d7b5bc8c8391dc7a61a678e6756d1))
- *(notifications)* Add dry-run digest subscription gate ([dec21f3](https://github.com/Ghent/capacitarr/commit/dec21f3f29335e4b2ab976b560b57e78add1e602))
- *(db)* Resolve SQLite 'database is locked' errors with WAL mode ([10ec06f](https://github.com/Ghent/capacitarr/commit/10ec06f2115932b359febc6b87c7fe81b192694c))
- *(ci)* Use correct trivy image from ghcr.io ([a574858](https://github.com/Ghent/capacitarr/commit/a574858b1a1fbbb605df9eae09af8349038f8ab1))

### 🛡️ Security

- *(phase-8)* Complete Phase 8 polish, testing & documentation ([2bbce07](https://github.com/Ghent/capacitarr/commit/2bbce07a775050ddb3211d08957e785115a4123c))
- *(security)* Add pre-release OWASP ZAP baseline for v2.0.0 ([14a28fc](https://github.com/Ghent/capacitarr/commit/14a28fc36416916f54df3f16b4bf63f3c5dad1ba))

### Refactor

- *(integrations)* [**breaking**] Rename overseerr → seerr ([ebe7cc5](https://github.com/Ghent/capacitarr/commit/ebe7cc582f6669517366874814797842a14ccc54))
## [1.10.0] - 2026-03-18

### 🚀 Features

- *(approval)* Add dismiss and clear queue endpoints ([2eae821](https://github.com/Ghent/capacitarr/commit/2eae821920c126a1ee854519de0997e4dc606eab))
- *(approval)* Allow force-delete in dry-run mode ([29a54b2](https://github.com/Ghent/capacitarr/commit/29a54b20f61c5f12cf239c8b6b9c5e3366b6c00a))
- *(preview)* Remove deleted items from library in real-time via SSE ([0c94fcf](https://github.com/Ghent/capacitarr/commit/0c94fcf4d2eda9f4443b1e3590566daf498f24a5))
- *(preview)* Add queue status indicators to library and deletion priority views ([8982f0d](https://github.com/Ghent/capacitarr/commit/8982f0d6d39587fd34fe676da73ad83c828d218e))
- *(deletion)* Add queue cancellation and listing API ([f1a1bbe](https://github.com/Ghent/capacitarr/commit/f1a1bbe95d9bdc11d180cefed56a4801304cbc60))
- *(ui)* Split approval queue into separate approval and deletion cards ([1894f92](https://github.com/Ghent/capacitarr/commit/1894f9236007ef680f396342ee6fccabce65ce41))
- *(ui)* Replace poster queue status pill with full-width banner ([01103f1](https://github.com/Ghent/capacitarr/commit/01103f1032144b2d9eaa05f98af05c1bced8a8c5))

### 🐛 Bug Fixes

- *(security)* Override h3 to >=1.15.6 for SSE injection and path traversal CVEs ([218da0e](https://github.com/Ghent/capacitarr/commit/218da0e6129b8a63d1bc1ec2b09219eb43aa2836))
## [1.9.0] - 2026-03-17

### 🚀 Features

- *(rules)* Add rule filter and manual force-delete ([59fa846](https://github.com/Ghent/capacitarr/commit/59fa846037ae8cb32b37c25d8630d3fdb5a9841d))
- *(library)* Add Library Management page with force-delete UI ([ba0affd](https://github.com/Ghent/capacitarr/commit/ba0affd3336fa65435853d5478ad58fb0804c565))
- *(library)* Replace pagination with virtual scrolling and add sort controls ([d62caf4](https://github.com/Ghent/capacitarr/commit/d62caf42a2e2fd1652c3e36306456a9ba7b704c8))
- *(settings)* Expand poll interval options from 1m to 24h ([4a12385](https://github.com/Ghent/capacitarr/commit/4a123858577303e72081cd0364299985ed428502))

### 🐛 Bug Fixes

- *(rules)* Use model-value for checkbox state in rule filter ([89599c4](https://github.com/Ghent/capacitarr/commit/89599c44b4537ec44d4d669c93630f73c3b5ba2a))
- *(rules)* Add selection UI to both grid and table view modes ([6e992da](https://github.com/Ghent/capacitarr/commit/6e992da7476e1e9971da52f7b725d118ff69148e))
- *(integrations)* Map tmdbId, originalLanguage, and releaseDate from *arr APIs ([fcba26a](https://github.com/Ghent/capacitarr/commit/fcba26aded8c6f2c2f1bb7c0522b48afce71e9e1)) — reported by @avikingr ([#2](https://github.com/Ghent/capacitarr/issues/2))
- *(scoring)* Normalize all ratings to 0–10 scale ([11cf6de](https://github.com/Ghent/capacitarr/commit/11cf6de38e26fe5abfc82ee377ee594a51051f1d))
- *(library)* Use UiDialog instead of missing UiAlertDialog for force-delete confirmation ([03e914d](https://github.com/Ghent/capacitarr/commit/03e914d895a02fb9cab0c67724e1841180578aa8))
## [1.8.0] - 2026-03-16

### 🚀 Features

- *(ci)* Add Discord release notification and settings tab persistence ([ee5b8f7](https://github.com/Ghent/capacitarr/commit/ee5b8f70768fadd7326cf7deb7d0fe6343b0a8a6))

### 🐛 Bug Fixes

- *(ci)* Remove static release header so git-cliff notes are used ([81fcd7f](https://github.com/Ghent/capacitarr/commit/81fcd7f9284b88a338e08976f74df10440a27fa8))
- *(ui)* Set page title to Capacitarr ([603a0c0](https://github.com/Ghent/capacitarr/commit/603a0c0ac09c810563f9957526cd9dcbed4b262c))
## [1.7.0] - 2026-03-16

### 🚀 Features

- *(backup)* Overhaul import robustness with upsert, validation, and interactive resolution ([17d2915](https://github.com/Ghent/capacitarr/commit/17d2915f5223aaab4df66503367f1d9405e515f4))

### 🐛 Bug Fixes

- *(ui)* Use modelValue instead of checked for reka-ui v2 Checkbox ([2eb84e9](https://github.com/Ghent/capacitarr/commit/2eb84e9814cf0c5a05258311d7e410b6732bde59))
- *(login)* Remove misleading placeholder text from login form ([5c2cf71](https://github.com/Ghent/capacitarr/commit/5c2cf71f4781c2b1d3871635b966d269dd2418fe))

### 🛡️ Security

- Comprehensive code audit — service layer, modularization, consistency ([31776b7](https://github.com/Ghent/capacitarr/commit/31776b7e1d4b1ba710a2cbfeb3e37dc3c0384254))
- *(security)* Update ZAP DAST baseline to 2026-03-16 ([87b24d0](https://github.com/Ghent/capacitarr/commit/87b24d058e985664e097f8567b314aa97e7fc63e))
## [1.6.0] - 2026-03-16

### 🚀 Features

- *(disk-groups)* Add user-defined disk size override ([31c8f9c](https://github.com/Ghent/capacitarr/commit/31c8f9c0e4344a120f24114904b335dce91ba13b))
- *(dashboard)* Add contextual empty states and integration error banner ([07929f8](https://github.com/Ghent/capacitarr/commit/07929f8495671daf864f85dde03c5b3ce94301a3))
- *(rules)* Add PB (petabyte) unit option for disk size override ([248a889](https://github.com/Ghent/capacitarr/commit/248a8898e4075dcfee0c23121bcfef925b596e8b))
- *(disk-groups)* Extract DiskGroupService, add integration badges, fix orphan cleanup ([fda8c00](https://github.com/Ghent/capacitarr/commit/fda8c005cd1d96c363e20f0204ba422b529b80b1))

### 🐛 Bug Fixes

- *(dashboard)* Compact override input, auto-save clear, error banner redesign ([12f86cb](https://github.com/Ghent/capacitarr/commit/12f86cb4b4fe675c3f0acdb6d39c99d939db5ca5))
- *(dashboard)* Move error banner to app layout, verify hero cards ([650c155](https://github.com/Ghent/capacitarr/commit/650c155c576f206787e473c0ca5bc5f94c6afcdb))
- *(settings)* Fix GORM nil handling when clearing disk size override ([2b4bb01](https://github.com/Ghent/capacitarr/commit/2b4bb010242dbe6ccf6c943d6b8f0d349991f2f4))
- *(rules)* Fix override clear not updating UI without page refresh ([5ea1aac](https://github.com/Ghent/capacitarr/commit/5ea1aac3cc76729fcbc80a9319b286654a70e9d4))
- *(ui)* Move integration error banner below page titles ([a415390](https://github.com/Ghent/capacitarr/commit/a415390152a1667a5855bf60e5bfa1fa13691cb4))
- *(poller)* Clean orphan disk groups when all integrations fail ([f4c134b](https://github.com/Ghent/capacitarr/commit/f4c134b2eebe00c20a845478ae6906a3e27f6069))
- *(integrations)* Clear stale error on update, refresh banner via SSE ([b746345](https://github.com/Ghent/capacitarr/commit/b746345afaf5d57afe0618a41747c1cadb94205c))
## [1.5.3-rc.3] - 2026-03-15

### 🐛 Bug Fixes

- *(site)* Resolve mermaid diagram overlap on pages with multiple diagrams ([34a8cca](https://github.com/Ghent/capacitarr/commit/34a8cca68d17ea003255fa562a0e394129000601))
- *(site)* Replace ELK layout engine with dagre to fix diagram overlap ([67af9fd](https://github.com/Ghent/capacitarr/commit/67af9fdda18fff59faac28d970c05d414ae1ae2f))
- *(poller)* Normalize Windows backslash paths from *arr APIs ([75839c2](https://github.com/Ghent/capacitarr/commit/75839c2b5ef1c95dc6312216cf1176fbef57a80e))
## [1.5.2] - 2026-03-10

### 🐛 Bug Fixes

- *(docs)* Remove breakout layout, fix edge label pills, split large diagrams ([d7719cf](https://github.com/Ghent/capacitarr/commit/d7719cfac6c00b4e478a5fefc7814b5054c65075))
## [1.5.1] - 2026-03-10

### 🐛 Bug Fixes

- *(site)* Rename ProseCode to ProsePre for Nuxt UI v4 and strip duplicate headings ([64a352e](https://github.com/Ghent/capacitarr/commit/64a352ef64ccdfc39e5fbbfe8f3488299ce77b57))
- *(site)* Scale mermaid diagrams to full width and remove duplicate descriptions ([16aab9e](https://github.com/Ghent/capacitarr/commit/16aab9e25caf1e30213014725b08cdd10bc138aa))
- *(site)* Strip inline SVG dimensions for responsive mermaid diagrams ([3af3462](https://github.com/Ghent/capacitarr/commit/3af3462fe5119a817b86c8d37e39b84fc19ca5eb))
- *(security)* Add nonce-based CSP for inline scripts ([7769920](https://github.com/Ghent/capacitarr/commit/776992013638c76a94a85fb751d4629ae4023be8))
- *(site)* Restore code block styling and add mermaid breakout layout ([c0eed01](https://github.com/Ghent/capacitarr/commit/c0eed01d04a44eaeaa800ed2608957a8aa3cc43a))
## [1.5.0] - 2026-03-10

### 🚀 Features

- *(deletion)* Add SSE deletion progress events ([15e9741](https://github.com/Ghent/capacitarr/commit/15e97412627fa295e37276b315139ec594d4d101))
- *(ui)* Add real-time deletion progress indicator and sparkline updates ([ec52ce1](https://github.com/Ghent/capacitarr/commit/ec52ce142c62aa509e7560a0acb82c7eea80ddd2))
- *(notifications)* [**breaking**] Add Apprise support and remove Slack ([fd255df](https://github.com/Ghent/capacitarr/commit/fd255df5ac366116c881d1dc9c502dfb28220dc7))
- *(ui)* Add Apprise notification channel support and remove Slack ([5100fd1](https://github.com/Ghent/capacitarr/commit/5100fd155dd52ba97380443d6f67b6d0439b8d10))
- *(backup)* Add settings export/import and remove rules portability ([ddd1c04](https://github.com/Ghent/capacitarr/commit/ddd1c04546e27a0abfdc7e20e529fb582e9ce1a9))
- *(ui)* Add settings backup/restore and remove rules import/export ([39bcf31](https://github.com/Ghent/capacitarr/commit/39bcf3105184bd513ef7f7940771f503497d913e))
- *(enrichment)* Add watchlist/favorites enrichment from Plex, Jellyfin, and Emby ([2ecb823](https://github.com/Ghent/capacitarr/commit/2ecb8236a4e16863bddb74e36242ae93eb19ca80))
- *(rules)* Add collection name rule field with autocomplete ([81a35ef](https://github.com/Ghent/capacitarr/commit/81a35ef79541cd18a0e84c886dd3f14b688fc08d))
- *(pwa)* Add Progressive Web App support for mobile home screen install ([02ca05a](https://github.com/Ghent/capacitarr/commit/02ca05a553d578d49df6b43e7a35e68c0723a39e))
- *(ui)* Add per-integration scoring weight override mockup ([b245527](https://github.com/Ghent/capacitarr/commit/b2455278c10006d2a40d2ff562e3ebf4e13053f1))
- *(site)* Add sidebar navigation ordering and fix security/ naming conflict ([77da624](https://github.com/Ghent/capacitarr/commit/77da624ae625ca14be046a0fffa19307b532f85b))

### 🐛 Bug Fixes

- *(ci)* Increase Node.js heap for pages job ([6dd994e](https://github.com/Ghent/capacitarr/commit/6dd994ebc374c1723242e64a5d75f4ba56942619))
- *(site)* Badge spacing, grouping, and duplicate heading on docs page ([1f3afb4](https://github.com/Ghent/capacitarr/commit/1f3afb4a640a58870778f2341fc9a45337ba669b))
- *(site)* Center badges, add donation hearts, add custom favicon ([933ebbe](https://github.com/Ghent/capacitarr/commit/933ebbe8d53936d49c23c7ee276937484facf341))
- *(site)* Badge centering, duplicate header, favicon 404 ([fa0dac3](https://github.com/Ghent/capacitarr/commit/fa0dac3f6890f7ed2cb02fb3c85b009c88dd6f40))
- *(engine)* Attribute approval-mode deletions to engine run stats ([98a81c9](https://github.com/Ghent/capacitarr/commit/98a81c952019b725b6dea8586b0d9bd6272bde13))
- *(ci)* Harden lint config, add typecheck, make security scans blocking ([b67d388](https://github.com/Ghent/capacitarr/commit/b67d3882995c0b160218e28d1f3bd469a1be1151))

### 🛡️ Security

- *(docker)* Harden Alpine runtime image ([25dc821](https://github.com/Ghent/capacitarr/commit/25dc8213e44db9ef3f63bc87d14ee4229741fed6))
- *(security)* Add security headers, scanning tools, response limits, clean tests ([df9b6e8](https://github.com/Ghent/capacitarr/commit/df9b6e84ef11a67f2d78b0f46f742d94035a18ed))
- *(security)* Add Trivy image scan, security regression tests, test server hardening ([922bbdf](https://github.com/Ghent/capacitarr/commit/922bbdf5d042902e08e87dfb99f4763e57d88f24))
- *(security)* Add OWASP ZAP DAST scanning ([51c268b](https://github.com/Ghent/capacitarr/commit/51c268b86afb4ae4c63f20c1118ea0e218c515bd))
- *(security)* Add ZAP baseline report, DAST section, security badge ([d2877d1](https://github.com/Ghent/capacitarr/commit/d2877d1846befd811caa9137dc176fbfa71e480c))
- *(site)* Auto-discover docs and sync root project files ([0fa9474](https://github.com/Ghent/capacitarr/commit/0fa9474131ce63b3d8272496ee7257aa35fe3ff7))
## [1.4.0] - 2026-03-09

### 🚀 Features

- *(site)* Add mermaid rendering, search, and screenshot refresh ([eb9c5d6](https://github.com/Ghent/capacitarr/commit/eb9c5d6e40c6bcddff64eaacf64cd700e8f1cdc4))

### 🐛 Bug Fixes

- *(auth)* Align cookie path with BASE_URL for subdirectory deployments ([29a9a9a](https://github.com/Ghent/capacitarr/commit/29a9a9aaf0229899060761ca24b79246a5867f77))
- *(ui)* Sort seasons by number instead of lexicographically ([9085445](https://github.com/Ghent/capacitarr/commit/9085445f9a258643ac25678016eee51775274f93))
## [1.3.0] - 2026-03-08

### 🚀 Features

- *(ui)* Add donation popover to app navbar ([593fb69](https://github.com/Ghent/capacitarr/commit/593fb69ce84585d8533c151f27ed376e423b3e0d))
- *(ui)* Use random cat/dog icon for donation button ([71ca3eb](https://github.com/Ghent/capacitarr/commit/71ca3eb0b19489e4825048d5409a78234298205b))

### 🐛 Bug Fixes

- *(site)* Badges, about section, footer icons, donation popover ([4bb429a](https://github.com/Ghent/capacitarr/commit/4bb429ae7237a6e39d21e537ac9eea90db601b6e))
- *(auth,approval)* BASE_URL login redirect and approval queue threshold clearing ([e5a34ac](https://github.com/Ghent/capacitarr/commit/e5a34ac2d2d2a41d8814f7a7c07a794e2332e2a2))
- *(ci)* Disable BuildKit provenance to remove unknown/unknown manifest entry ([e61a090](https://github.com/Ghent/capacitarr/commit/e61a09075c457927eccb7b57cceebed6e5f7c2a4))
## [1.2.2] - 2026-03-08

### 🐛 Bug Fixes

- *(ci)* Use alpine+crane for Docker mirror jobs ([b3548e0](https://github.com/Ghent/capacitarr/commit/b3548e07e59b6a8499198707db8c666da986a714))
## [1.2.1] - 2026-03-08

### 🐛 Bug Fixes

- *(ci)* Use POSIX sh for Docker CI scripts ([02eab40](https://github.com/Ghent/capacitarr/commit/02eab403726c863a111981df1d6e383a4e71d3ec))
## [1.2.0] - 2026-03-08

### 🚀 Features

- *(ci)* Add multi-registry Docker publishing to Docker Hub and GHCR ([fa3e1ac](https://github.com/Ghent/capacitarr/commit/fa3e1acaad0f00bb80e321f0941c3bc324826f5e))

### 🐛 Bug Fixes

- *(docs)* Constrain shields.io badge sizing on pages site ([e9f6579](https://github.com/Ghent/capacitarr/commit/e9f6579f8728c3d06868b64258596a2e3ba43192))
- Runtime subdirectory reverse proxy support via HTML rewriting ([a1668f2](https://github.com/Ghent/capacitarr/commit/a1668f20c08dae92595b72ea36bc7e89dc96c367))
- *(frontend)* Resolve subdirectory proxy cosmetic issues ([95c974a](https://github.com/Ghent/capacitarr/commit/95c974a528240ceabddbb935c7ea80ed6724690b))
## [1.1.0] - 2026-03-08

### 🚀 Features

- Fix engine mode switching, add social links, notification triggers, shields.io badges ([6494d44](https://github.com/Ghent/capacitarr/commit/6494d4412354bb13b260cbe0d035361ce1bcb6c5))
## [1.0.0] - 2026-03-07

### 🚀 Features

- *(notifications)* Make in-app notifications always-on ([735c234](https://github.com/Ghent/capacitarr/commit/735c2349c40e1ad7948b16744d803fa1059ca795))
- Add features and polish (Phase 3) ([5705d53](https://github.com/Ghent/capacitarr/commit/5705d53b9fbfcd78a66b358bbaefb7e823066862))
- Complete 1.0.0 pre-release cleanup (phases 2-5) ([08ed33f](https://github.com/Ghent/capacitarr/commit/08ed33f6d2f4ffab6d6eadf38e5537e4b87bbfc9))
- Complete all remaining plan steps ([6b9907a](https://github.com/Ghent/capacitarr/commit/6b9907a309b23ecab3bd223c49ad22e6162f601a))

### 🛡️ Security

- 1.0.0 pre-release cleanup ([14cae5d](https://github.com/Ghent/capacitarr/commit/14cae5d162d19848332a695057f0676ee182d7f0))
## [1.0.0-rc.12] - 2026-03-07

### 🐛 Bug Fixes

- *(notifications)* Gate approval-mode cycle digest by OnApprovalActivity ([53434a9](https://github.com/Ghent/capacitarr/commit/53434a932f0851b0ade8aca9e74ad795c8cc4137))
## [1.0.0-rc.11] - 2026-03-07

### 🚀 Features

- *(notifications)* Add event system foundation for notification overhaul ([9a8e319](https://github.com/Ghent/capacitarr/commit/9a8e319faee65604768f0bfd5854555dd7445c86))
- *(notifications)* Implement notification overhaul (Phase 1.6-7) ([a50a8e6](https://github.com/Ghent/capacitarr/commit/a50a8e644c4ab2a74f047b7ca33038b2e1c06825))
- *(notifications)* Add approval activity toggle and toggle descriptions ([a5e7cf6](https://github.com/Ghent/capacitarr/commit/a5e7cf61db65d3f1b6907e3ab6c3ade6bf3d216b))

### 🐛 Bug Fixes

- *(frontend)* Remove @click.prevent on MediaPosterCard component emits ([a6aafb4](https://github.com/Ghent/capacitarr/commit/a6aafb4925d89c84784684dc1ad9c7ebcdf304d8))
- *(notifications)* Persist OnApprovalActivity toggle and report freed bytes in approval mode ([94a4121](https://github.com/Ghent/capacitarr/commit/94a412124627e2174f266dd271f2d3ca4f8f1347))

### 🛡️ Security

- *(plans)* Mark service layer remediation plan as complete (Phases 7-10) ([b4a3d9d](https://github.com/Ghent/capacitarr/commit/b4a3d9d8250e34a6ebf850b82facb951dbbe9854))
## [1.0.0-rc.9] - 2026-03-07

### 🚀 Features

- *(ui)* Add NumberField and Combobox shadcn-vue components ([805d893](https://github.com/Ghent/capacitarr/commit/805d893b4fca2e62520f27fa1d4e1088c4f877be))
- *(rules)* Add custom rules import/export ([b3e8a35](https://github.com/Ghent/capacitarr/commit/b3e8a352cdb104f8a0cb3f2c4b681fe5e99c3393))

### 🐛 Bug Fixes

- *(ui)* Use zone colors on threshold slider instead of primary gradient ([50be5e4](https://github.com/Ghent/capacitarr/commit/50be5e45b3177b71baf697e7c9acbaa46090307c))
- *(ui)* Raise slider thumb z-index above zone color overlays ([36c8f94](https://github.com/Ghent/capacitarr/commit/36c8f9470f8692f72fd399c00cc0cddc73016076))
- *(ui)* Correct target thumb selector and enlarge threshold thumbs ([1669982](https://github.com/Ghent/capacitarr/commit/1669982560648c0fe6a6070d6e386e74acda313e))
- *(security)* Upgrade Go 1.25 → 1.26 to resolve 4 stdlib vulnerabilities ([3ab190c](https://github.com/Ghent/capacitarr/commit/3ab190c990632a1214a205c0271fd57419af2a07))

### ⚡ Performance

- *(ci)* Add Docker volume caching for Go and Node dependencies ([7709e63](https://github.com/Ghent/capacitarr/commit/7709e6328cb285c7d57e336ba214ec19e081f1cb))

### 🛡️ Security

- Add make ci gate to release script ([1ad7882](https://github.com/Ghent/capacitarr/commit/1ad7882ceb9ba968d2a19618bbca1289752692bd))
## [1.0.0-rc.8] - 2026-03-06

### 🐛 Bug Fixes

- *(lint)* Use NewRequestWithContext in test files ([277f558](https://github.com/Ghent/capacitarr/commit/277f55809d606350501ca672f75b412f16699b1f))
## [1.0.0-rc.7] - 2026-03-06

### 🚀 Features

- Add poster URL plumbing for grid view (Phase 1) ([5f01060](https://github.com/Ghent/capacitarr/commit/5f01060c4e8099eaef832cea82d6d21e5cc317f9))
- *(frontend)* Add grid/list view toggle with poster cards (Phase 2) ([72eedd8](https://github.com/Ghent/capacitarr/commit/72eedd8f4fc98273070cdfe954b7e199fa4ff4d9))
- *(frontend)* Add selection checkboxes and season badges to grid cards (Phase 3) ([7fbcd58](https://github.com/Ghent/capacitarr/commit/7fbcd585e244c2d7ffb16a80256612164fdd11cb))
- *(frontend)* Add season popover for show cards in grid view (Phase 3) ([dcc2e9d](https://github.com/Ghent/capacitarr/commit/dcc2e9db993ce4ddd896f7da16f605bd9d7221b3))
- *(frontend)* Add deletion line divider and season popovers to preview grid ([50ecccb](https://github.com/Ghent/capacitarr/commit/50ecccbe18c43661b140645145eeae68c0b48a65))
- *(enrichment)* Add Plex as watch history enrichment source ([31c44b4](https://github.com/Ghent/capacitarr/commit/31c44b41dcd40324ccbc4f03c064ef1de1dad8cf))
- *(version)* Add Check Now button to update popup ([bf72c7a](https://github.com/Ghent/capacitarr/commit/bf72c7a4da4981ec0353d0c57f786139daa5d443))

### 🐛 Bug Fixes

- *(frontend)* Reposition card overlays to avoid title overlap ([605a817](https://github.com/Ghent/capacitarr/commit/605a81708c5530297d5a1a8fb243ca17ff886c34))
- *(frontend)* Snoozed grid unsnooze actions and preview infinite scroll ([2fa8bf2](https://github.com/Ghent/capacitarr/commit/2fa8bf256fe52ecb114cabb4fbe652936e1d86b0))
## [1.0.0-rc.6] - 2026-03-06

### 🚀 Features

- *(events)* Add event bus infrastructure and 34 typed event structs ([b284237](https://github.com/Ghent/capacitarr/commit/b28423748cf693e61dd910a5aa7d8a54ecce9fa7))
- *(events)* Add activity persister subscriber ([f830168](https://github.com/Ghent/capacitarr/commit/f830168471e17fc74d4cf5acbbc5131e5516a039))
- *(services)* Add core service layer — ApprovalService, DeletionService, AuditLogService, EngineService ([101082f](https://github.com/Ghent/capacitarr/commit/101082f62dc5d317c8789923408890b71c87c4c0))
- *(services)* Add secondary services and registry ([6b91961](https://github.com/Ghent/capacitarr/commit/6b9196140eab1e54c0906ea506010aa25cb0e641))
- *(events)* Add SSE broadcaster for real-time event streaming ([8905f0e](https://github.com/Ghent/capacitarr/commit/8905f0efc873b8e38f6de0b418cdfcac42a24fd9))
- Add frontend SSE composable, activity pruning to 7-day retention ([86002a9](https://github.com/Ghent/capacitarr/commit/86002a9f2586a1f7ef659a9e033f7418a787125c))
- *(notifications)* Add event bus subscriber for notification dispatch ([d1a9cc5](https://github.com/Ghent/capacitarr/commit/d1a9cc5552ab19d06d057e791b7ded8ebfd15074))
- *(frontend)* Update types and API endpoints for new schema ([d00c7ad](https://github.com/Ghent/capacitarr/commit/d00c7adee79acf2ff6d2105772cf7a2a2e863eea))
- *(frontend)* Add icon/color mapping for all 34 event types ([1e35936](https://github.com/Ghent/capacitarr/commit/1e35936a42be06ee24850d5501e6f644db5bfc0a))
- *(approval)* Add section jump navigation to approval queue ([197d716](https://github.com/Ghent/capacitarr/commit/197d716abbf90f16c44ebcfc6fc02aeb08d3e676))

### 🐛 Bug Fixes

- *(events)* Fix deadlock in concurrency stress test ([00b50c1](https://github.com/Ghent/capacitarr/commit/00b50c18253464657b00f57800a35ebe4687db97))

### Refactor

- *(db)* [**breaking**] Replace 18 incremental migrations with single clean baseline ([fafa409](https://github.com/Ghent/capacitarr/commit/fafa4094bc073f6b8b36ede7dba8ff83485dcdf8))
## [1.0.0-rc.5] - 2026-03-05

### 🚀 Features

- *(navbar)* Always-visible update icon with breathing animation ([b0c0980](https://github.com/Ghent/capacitarr/commit/b0c0980d5794371688fd531fb708724c017a02ad))

### 🐛 Bug Fixes

- *(test)* Resolve flaky test failures in routes package ([14d3e08](https://github.com/Ghent/capacitarr/commit/14d3e0827a3fcec725f5eeb9794bd03471bf3c92))
- *(dashboard)* Shrink activity scroll area to match sparkline height ([65ebcfa](https://github.com/Ghent/capacitarr/commit/65ebcfa388900ba9ba83093717d59e3c36135e64))
- *(dashboard)* Constrain activity scroll area height properly ([18f6579](https://github.com/Ghent/capacitarr/commit/18f6579cc0f92fdb530f8d9428a82a1075d11088))
- *(plex)* Use getRandomValues for UUID in non-secure contexts ([d281d5a](https://github.com/Ghent/capacitarr/commit/d281d5a74e4a0d69a88f49a0ca18b889cf8be47e)) — reported by @wulfe ([#1](https://github.com/Ghent/capacitarr/issues/1))
## [1.0.0-rc.4] - 2026-03-05

### 🚀 Features

- *(version)* Add update check endpoint with 6h cache ([2adf50c](https://github.com/Ghent/capacitarr/commit/2adf50ce762dfabf7ed34df3411273276746d09e))
- *(navbar)* Add update check indicator and Serenity slogan ([01fc236](https://github.com/Ghent/capacitarr/commit/01fc236651e4a48f705d371748efd245810303f0))
- *(engine)* Track deleted count per run with run-stats-ID approach ([a00b42e](https://github.com/Ghent/capacitarr/commit/a00b42efa1dea383214014dd6c94274bb19d1efe))
- *(engine)* Add history endpoint, remove audit/activity ([7b6f708](https://github.com/Ghent/capacitarr/commit/7b6f7080e372ce56b96760ab7739768180d1aa4e))
- *(dashboard)* Consolidate sparklines onto engine history data ([ba92422](https://github.com/Ghent/capacitarr/commit/ba92422611115ccd44930f05f5c73f216ed418df))
- *(approval)* Block approvals when deletions disabled, add orphan recovery ([cf9a3e5](https://github.com/Ghent/capacitarr/commit/cf9a3e56f402d065e935bbf0be9ad092d24a64b7))

### 🐛 Bug Fixes

- *(deps)* Override svgo and tar to resolve pnpm audit vulnerabilities ([7c20356](https://github.com/Ghent/capacitarr/commit/7c203561e6e09b3a48f64aecea900214d62f6d70))
- *(data)* Preserve disk group thresholds during data reset ([ea7f73b](https://github.com/Ghent/capacitarr/commit/ea7f73be287642ec5d76e8fee371b7a7f843e8af))
- *(frontend)* Replace bare catch blocks with console.warn logging ([b653de0](https://github.com/Ghent/capacitarr/commit/b653de0a30c4a93a5a0b14bf325f4f30fe80757c))
- Use Find+Limit instead of First for optional queries ([0c51f8b](https://github.com/Ghent/capacitarr/commit/0c51f8b2ef1cc8d41da0cf93a9ce448f881eea31))
- *(dashboard)* Use dateRange dropdown for sparkline labels and improve color contrast ([9c03735](https://github.com/Ghent/capacitarr/commit/9c037353fbc14c97858c8c39f0821a9034df8102))
- *(navbar)* Inline Serenity SVG so currentColor inherits text color ([837c651](https://github.com/Ghent/capacitarr/commit/837c651ad2a53103ef561443e005d633382a5eb4))
- *(dashboard)* Display sparkline timestamps in browser local timezone ([43edd0d](https://github.com/Ghent/capacitarr/commit/43edd0d10d27c4ece21cf5277f07af6c62eca528))
- Sparkline accuracy, tooltips, and visual quality ([e48d60c](https://github.com/Ghent/capacitarr/commit/e48d60c38cfe36e9de64cf9ba1322e2fbc4270c2))
## [1.0.0-rc.3] - 2026-03-05

### 🐛 Bug Fixes

- Resolve golangci-lint issues and align local linting with CI ([975bf6d](https://github.com/Ghent/capacitarr/commit/975bf6d6d076e32eac5e770fdc601a897b8b3b7f))
## [1.0.0-rc.2] - 2026-03-05

### 🚀 Features

- *(ui)* Truncate API keys and reposition effect badge ([23a98e4](https://github.com/Ghent/capacitarr/commit/23a98e43b744a8f13af808e776edc9098d8a427d))
- *(auth)* Add auth status endpoint and first-login setup UX ([ec7f68a](https://github.com/Ghent/capacitarr/commit/ec7f68a112285af7e5a0724bd034f85ee2660edd))
- *(ui)* Add DateDisplay component with date toggle and settings control ([c69af7e](https://github.com/Ghent/capacitarr/commit/c69af7eb1fa3ece45969e1d302e4ac3a7b703809))
- *(plex)* Reimplement OAuth flow client-side and remove backend proxy ([a239c73](https://github.com/Ghent/capacitarr/commit/a239c73b588c3f7051602ddbd289d89347ab2ec8))
- *(rules)* Add lastplayed, requestedby, incollection, watchedbyreq rule fields with date-aware operators ([75b787d](https://github.com/Ghent/capacitarr/commit/75b787db75eb17bf64964e1f326906a18d6d87ae))
- *(approval)* Add approval queue with approve/reject endpoints and UI column ([d707eaf](https://github.com/Ghent/capacitarr/commit/d707eafd3028cae2094b25d659802ef0ea4ec3f9))
- *(approval)* Add snooze mechanism with configurable duration and auto-clear ([48dab85](https://github.com/Ghent/capacitarr/commit/48dab850f3c358ab6c202cc87bd1f6a6dac6887f))
- *(approval)* Add snooze states and undo UI for approval workflow ([8fa13f5](https://github.com/Ghent/capacitarr/commit/8fa13f5491b5d62883f9cad51db9b80087c843dc))
- Readarr full support, fix undo/run-now/capacity-chart, approval card enhancements ([23a95ff](https://github.com/Ghent/capacitarr/commit/23a95ff9923e9885cd8318af84228b81461298e9))
- *(approval)* Move checkboxes to right of snooze icon and add season-level selection ([392b8f8](https://github.com/Ghent/capacitarr/commit/392b8f8adfa48469ca18d9e56f32c9c0a1d90321))
- *(approval)* Add approve/snooze buttons to individual season rows ([e045bc8](https://github.com/Ghent/capacitarr/commit/e045bc81b3bc1bf8273b0b32fa31dae4eb600e52))
- *(engine)* Prefer season-level audit entries over show-level for granular approval ([148268c](https://github.com/Ghent/capacitarr/commit/148268c3bb0757b209b823339adbfac047315d52))
- *(site)* Add GitLab repo stats widget to header ([df19889](https://github.com/Ghent/capacitarr/commit/df19889d90bc379a0871d8549ad34c0b9479d3d1))

### 🐛 Bug Fixes

- Resolve TypeScript strict mode issues from Phase 7 review ([be6399d](https://github.com/Ghent/capacitarr/commit/be6399df4a27098beb277d566e3f47bc2322af4a))
- *(i18n)* Disable optimizeTranslationDirective to suppress deprecation warning ([8a45e67](https://github.com/Ghent/capacitarr/commit/8a45e677cbc99164ef2efa52eeb15cfed3d4956e))
- *(rules)* Include lastplayed in conflict detection and map date-aware operators ([3c76126](https://github.com/Ghent/capacitarr/commit/3c76126a6b471e6ca098f2c8b0ac789d77c50332))
- *(rule-builder)* Replace combobox with free-text input and suggestion dropdown ([8a5a26f](https://github.com/Ghent/capacitarr/commit/8a5a26f06a6b75d729203a10b1153dc6113a7de0))
- *(settings)* Group exact dates toggle with timezone and clock format ([2f83b52](https://github.com/Ghent/capacitarr/commit/2f83b52ee95ca3e72e7a0323654e25e91700c35b))
- *(approval)* Reorder v-if chain to check snooze before pending approval ([9a8fe97](https://github.com/Ghent/capacitarr/commit/9a8fe97943654b9753ed092c65cdf6f10d7545c0))
- *(approval)* Simplify snooze display to compact icons without timestamp ([720bb22](https://github.com/Ghent/capacitarr/commit/720bb227f94dbbdfaebc3a2bd71b848788228c21))
- *(rules)* Skip snoozed items when calculating deletion line index ([14d25ee](https://github.com/Ghent/capacitarr/commit/14d25ee3d5936a780ebcdf3eb0472e19e8f6294d))
- Normalize dry_run to dry-run across frontend and backend ([6f7bd8c](https://github.com/Ghent/capacitarr/commit/6f7bd8c7b8c6a02385bc4503f2f6ad1ddf6502ab))
- *(settings)* Show masked API key placeholder when key exists but is hashed ([5b2002e](https://github.com/Ghent/capacitarr/commit/5b2002efdd4f75b436d44e63bf2edb892734746e))
- Standardize error responses, fix cache lifecycle, improve error logging ([77ce154](https://github.com/Ghent/capacitarr/commit/77ce154f9d5dd62811508e654bb44e94e16f9bfe))
- *(approval)* Show season approve/snooze buttons and align size column ([87f13e3](https://github.com/Ghent/capacitarr/commit/87f13e357b0a23c25898c6d833691bb3b1793f26))
- Correct site and docs content accuracy ([e116e51](https://github.com/Ghent/capacitarr/commit/e116e510f33a57f5f7bda2c842b44eced206cd23))
- *(site)* Replace Nuxt UI Docs TOC link with GitLab repo link ([e7c262d](https://github.com/Ghent/capacitarr/commit/e7c262d2e42f5ad8855284f9c95a0cd563dc07a9))
## [1.0.0-rc.1] - 2026-03-03

### 🐛 Bug Fixes

- *(deps)* Update golang.org/x/net to v0.51.0 (GO-2026-4559) ([164d22d](https://github.com/Ghent/capacitarr/commit/164d22d2a5c8779f326f89906ab117c8b54c1f08))
- Include package-lock.json and frontend version in release script ([37eabc2](https://github.com/Ghent/capacitarr/commit/37eabc28c38b4bed4cfa609b5aed935ecfab5871))
## [0.1.2] - 2026-03-03

### 🐛 Bug Fixes

- *(ci)* Add release_notes.md to .gitignore ([3bc13cc](https://github.com/Ghent/capacitarr/commit/3bc13cc1661f31c21dd5de938b87ed13a91c9b6d))
## [0.1.1] - 2026-03-03

### 🐛 Bug Fixes

- *(ci)* Use goreleaser v2 hook syntax (strings, not maps) ([e1db455](https://github.com/Ghent/capacitarr/commit/e1db4556d1f23b438f4d20e8c52a41caf63aebc4))
## [0.1.0] - 2026-03-03

### 🚀 Features

- Complete Phase 2 Core Backend Engine ([7f5ec46](https://github.com/Ghent/capacitarr/commit/7f5ec468af750136a35f39c8c20edf12c36097e0))
- Complete Phase 3 Data Aggregation & Logic ([e9de2cd](https://github.com/Ghent/capacitarr/commit/e9de2cdf79fb4b75fc2630ac1f702b23ce28479b))
- Complete Phase 4 Frontend Foundation ([cbbdceb](https://github.com/Ghent/capacitarr/commit/cbbdcebd26b2833c6797be3d5bd01d2bb8f6d5c7))
- Complete Phase 5 Premium Data Visualization ([9947f82](https://github.com/Ghent/capacitarr/commit/9947f820483f1774da5f80c8c17a2107aef7386f))
- Complete Phase 6 Deployment and multi-stage container ([3c04908](https://github.com/Ghent/capacitarr/commit/3c049087e67a9743ff48b8375cef6aefb9461206))
- Complete Phase 1 Real Data ([e2b2bfe](https://github.com/Ghent/capacitarr/commit/e2b2bfea29f59fe7bc06883531ba7ff0839eb5ce))
- *(db)* Replace AutoMigrate with Goose migration framework ([f3b296b](https://github.com/Ghent/capacitarr/commit/f3b296b461d67119ea6cbf407f7fc53abc35827a))
- Add reverse proxy & auth header support ([628f3e8](https://github.com/Ghent/capacitarr/commit/628f3e894ddb92c106d1286893e55cc4e75118a2))
- Add configurable poll interval and restructure settings into tabs ([c577844](https://github.com/Ghent/capacitarr/commit/c5778441336239353c27773955731145ac7a7cd1))
- *(auth)* Add password change endpoint ([13e8b89](https://github.com/Ghent/capacitarr/commit/13e8b896cec5cf2e26bcf3660ae7bf8c8be11f74))
- *(auth)* Add API key authentication support ([b7aae55](https://github.com/Ghent/capacitarr/commit/b7aae556ea371d7a608949bfa563d6e6fa7e5ed7))
- Wire integrations enrichment and add service-specific rule fields ([ecab144](https://github.com/Ghent/capacitarr/commit/ecab144f03a462b253a1c18ed786a1e161c207fc))
- *(audit)* Add show/season grouping with collapsible tree view ([e26b848](https://github.com/Ghent/capacitarr/commit/e26b848be48dc3736d1db7b15bfb79379a299447))
- *(ui)* Comprehensive visual design polish ([b0897d9](https://github.com/Ghent/capacitarr/commit/b0897d9e2a975347899fe84c1e3ded2515ecb08e))
- *(settings)* Add per-integration contextual help ([691ee23](https://github.com/Ghent/capacitarr/commit/691ee23c75ed8764e1d53fd5101a3243fdc5b261))
- Add branded splash screen during app load ([30fd8ce](https://github.com/Ghent/capacitarr/commit/30fd8ceec28d703ab7e8902554decc6f89b76b03))
- Add version display in navbar and API version endpoint ([4fc3b8b](https://github.com/Ghent/capacitarr/commit/4fc3b8b97cfe303e6cccc8d9184d56dcd3f55a7c))
- *(settings)* Refactor into 4 tabs with Advanced and Security ([157d2fc](https://github.com/Ghent/capacitarr/commit/157d2fcff174e80260eb0f455ebf829704aabe65))
- *(navbar)* Add engine control popover with mode toggle + run now ([150ef05](https://github.com/Ghent/capacitarr/commit/150ef05828482974b4db8e2a4a9322f5be9b7805))
- Add Readarr, Jellyfin, and Emby integration support ([acebf60](https://github.com/Ghent/capacitarr/commit/acebf60928fe6db02913b3c71e0bd876d1f97839))
- *(engine)* Redesign indicator to separate mode from status ([08ab89d](https://github.com/Ghent/capacitarr/commit/08ab89dc77f66ab430bafe15327d4da9c79b0cd6))
- *(engine)* Persist engine run stats to SQLite ([8aec436](https://github.com/Ghent/capacitarr/commit/8aec436797cd21eb06a0bce0ff2082446634f416))
- Implement Phase 1 cascading rule builder ([151c466](https://github.com/Ghent/capacitarr/commit/151c466a990df40db30c487dd394f375b352c265))
- *(rules)* Add value autocomplete and conflict indicators (Phase 2 + 3) ([7762f2f](https://github.com/Ghent/capacitarr/commit/7762f2f1c2f9533944cad48442c9c73ffa96353f))
- *(rules)* Add 'takes effect on next run' note to card description ([648b869](https://github.com/Ghent/capacitarr/commit/648b869eb18497bf6362b81766b0a2853bd391e0))
- *(rules)* Show service type in rule list (e.g. 'Radarr: selene') ([6fbfa52](https://github.com/Ghent/capacitarr/commit/6fbfa52d494df81581f83457c04be66ae6fd5b69))
- Add search and filter capabilities to Live Preview and Audit Log ([bfe1b43](https://github.com/Ghent/capacitarr/commit/bfe1b4388ec8003093ef2de0119cb74910501bcb))
- *(rules)* Improve effect color distinction with wider spectrum and emoji icons ([4eca960](https://github.com/Ghent/capacitarr/commit/4eca96049a2cc112e4d9898a96f85a8cc2dae901))
- *(tables)* Add sticky headers and sortable columns to preview and audit tables ([d77fe3b](https://github.com/Ghent/capacitarr/commit/d77fe3b8f9ec3fb73cd50e2c16dd8648716ea16e))
- Move execution mode and tiebreaker to settings, add preset descriptions ([4bc8a92](https://github.com/Ghent/capacitarr/commit/4bc8a92278a0a7b7882ccbacb4efc3e90f259098))
- *(preview)* Add deletion priority line and disk context to live preview ([298211f](https://github.com/Ghent/capacitarr/commit/298211fa6561550aeeadf4a740f2dd41fc111867))
- *(settings)* Add clear all scraped data button to Advanced tab ([8c95a08](https://github.com/Ghent/capacitarr/commit/8c95a08e62b380d68c9a0b8e5deb9dcbf9875a24))
- *(ui)* Increase navbar opacity, rename Availability to Series Status, add About popover ([549c249](https://github.com/Ghent/capacitarr/commit/549c249ba37c81ebbedf2ac0dfff007b43d84543))
- *(dashboard)* Reorder layout, enhance engine activity card ([3a54e10](https://github.com/Ghent/capacitarr/commit/3a54e1069eb826e7cd8fc69a9089e5a1d2dd3499))
- *(dashboard)* Add lifetime stats scorecards with cumulative counters ([2374f8b](https://github.com/Ghent/capacitarr/commit/2374f8b97322bfa89ede05ec9a828ac9520a13cf))
- Remove item limits, add progressive scroll, wire Jellyfin/Emby enrichment ([697b1b5](https://github.com/Ghent/capacitarr/commit/697b1b551131570e5043e4cd781e5bf7e9ed88e5))
- *(ui)* Add tagline to navbar brand ([a94a8dd](https://github.com/Ghent/capacitarr/commit/a94a8dd8649e22ed96e4d871b1259e11dd2d6988))
- *(security)* Add rate limiting to login endpoint ([6c8f63c](https://github.com/Ghent/capacitarr/commit/6c8f63cc36f96e9e13275f9c8e0dfde058cad5e3))
- *(logging)* Add component tags and structured error fields to all slog calls ([16c20fb](https://github.com/Ghent/capacitarr/commit/16c20fb9d938a074d438530b2cb8970713b285de))
- *(logging)* Add comprehensive debug logging to engine, integrations, and cache ([6b395cf](https://github.com/Ghent/capacitarr/commit/6b395cff7283bed03a7a54f662cc59c15e7c2314))
- *(logging)* Add request ID generation, propagation, and startup config logging ([3c7141d](https://github.com/Ghent/capacitarr/commit/3c7141de6824b9a551540b9d652269232b4790c5))
- *(api)* Add cleanup history endpoint for sparkline chart ([83beb40](https://github.com/Ghent/capacitarr/commit/83beb405fb631937bb62a3e82f3f49ea818894d4))
- *(ui)* Add cleanup sparkline chart to dashboard engine activity card ([4222db9](https://github.com/Ghent/capacitarr/commit/4222db99f8f3f3afeeafbba49b254462b3e49988))
- *(a11y)* Add ARIA labels, focus-visible styles, and semantic landmarks ([f7b075c](https://github.com/Ghent/capacitarr/commit/f7b075ce318d1af97edc1280ec8d0eaf4a1d594e))
- *(ui)* Add rule order disclaimer and navbar language selector ([e117c02](https://github.com/Ghent/capacitarr/commit/e117c02cf08876b353cba10b6f254826d8fc7754))
- *(i18n)* Add 18 new language translations and complete existing ones ([1533d57](https://github.com/Ghent/capacitarr/commit/1533d57bc7509df524fa7f6aa0946313345d16ab))
- Complete all remaining plan items ([344ae48](https://github.com/Ghent/capacitarr/commit/344ae481317ca2e05d9da2c4c0c44bc9e754a8b4))
- UI polish pass — conflict detection, connection banner, i18n, and UX improvements ([dd7a5d1](https://github.com/Ghent/capacitarr/commit/dd7a5d15586ea9cc75254e849c32ca45cd1b9bf6))
- Polish pass round 2 — deletion toggle fix, about card, backup docs ([855b222](https://github.com/Ghent/capacitarr/commit/855b222da7e2c091b16ebcc7da082256f26eba5f))
- Comprehensive UI polish pass ([be538ad](https://github.com/Ghent/capacitarr/commit/be538adc7cfe273ed0bb145f119d27b7efc281fa))
- *(site)* Replace MkDocs with Nuxt UI v4 project site ([5f97a4a](https://github.com/Ghent/capacitarr/commit/5f97a4ac3bc5884143827b86b2e13ff162903fe5))
- *(site)* Complete project site with visual polish and custom domain support ([f79562c](https://github.com/Ghent/capacitarr/commit/f79562c7ad18ff38216feda29f124bd2e0fd4aa6))
- Add linux/arm64 multi-arch Docker support ([ad85fc7](https://github.com/Ghent/capacitarr/commit/ad85fc72ebc48cb4ac6d5b355d146321d143c3b4))

### 🐛 Bug Fixes

- *(poller)* Add atomic concurrency guard to prevent overlapping poll runs ([edf9769](https://github.com/Ghent/capacitarr/commit/edf976974198ccd05c5a40717c7a59d1155afa4a))
- *(db)* Fix baseline migration for existing databases ([64ae743](https://github.com/Ghent/capacitarr/commit/64ae743255a60268b6f7d6562c9f8433aaadcc2c))
- *(fonts)* Use @fontsource/geist-sans and geist-mono instead of geist ([aea30a7](https://github.com/Ghent/capacitarr/commit/aea30a7739619f23623f69130c77fce75227c2d2))
- *(ui)* Fix score bars, slider track, card borders, and sparkline colors ([b41268c](https://github.com/Ghent/capacitarr/commit/b41268cc6c83725200c0ee55adcedfead5ea5288))
- *(ui)* Comprehensive visual polish overhaul ([a38f44e](https://github.com/Ghent/capacitarr/commit/a38f44e8fcac467b9e94ddb8af73188e92c90957))
- Resolve broken design system and add touch support ([2501d99](https://github.com/Ghent/capacitarr/commit/2501d99bc120ae6f39f366f748b3796f7e710c81))
- *(toast)* Raise z-index to z-[100] to render above dialogs ([8800fdc](https://github.com/Ghent/capacitarr/commit/8800fdcf25117e27bae2d4838c80fe08059cb4ca))
- Safety guards and UX feedback fixes ([94b52ec](https://github.com/Ghent/capacitarr/commit/94b52ec9725a3b6cadfe046b2a60dbdbce7df002))
- Engine mode switch, overseerr error msg, and follow-up fixes ([74a8f41](https://github.com/Ghent/capacitarr/commit/74a8f41774b9d9ac90515f02c3e90bc09b6661f0))
- *(db)* Add goose annotations to migration 00005 ([2ee5c66](https://github.com/Ghent/capacitarr/commit/2ee5c66224fa047f81759f6fd216aca449a17666))
- *(rules)* Add rule numbers and type prefix to service dropdown ([7e17199](https://github.com/Ghent/capacitarr/commit/7e171996b9c1032db4c65126a455d8acf3ae22a2))
- *(preview)* Improve show/season grouping and left-align season chevron ([9734aa7](https://github.com/Ghent/capacitarr/commit/9734aa7b6ff13a3b8c532880337b253948c3e68f))
- *(preview)* Hide show entries with no seasons in preview ([068cb84](https://github.com/Ghent/capacitarr/commit/068cb84d0475b1420290a22048e63c3ed298c6c3))
- *(preview)* Collapse seasons by default in live preview ([ceb29bd](https://github.com/Ghent/capacitarr/commit/ceb29bd4b5463726b71a62d5fb00d52e7762633a))
- *(poller)* Deduplicate audit log entries across engine runs ([5b5bb86](https://github.com/Ghent/capacitarr/commit/5b5bb86b6628c5333fcb2040729f30f9f5e5ca1a))
- *(audit)* Move season chevron next to title, matching live preview layout ([5e0444b](https://github.com/Ghent/capacitarr/commit/5e0444bf63f77a7bf683e4a15b97c49ce7893fe2))
- *(ui)* Unify disk group color logic between dashboard and scoring engine ([45b22bb](https://github.com/Ghent/capacitarr/commit/45b22bb616a5949642d353ef4c544b9c2a2ef31f))
- *(preview)* Dynamic item limit based on bytesToFree ([2b8498b](https://github.com/Ghent/capacitarr/commit/2b8498b74c6b73e0b7d67feaea63cec7c8576cb2))
- *(auth)* Increase bcrypt cost to 12 for stronger brute-force resistance ([99d4504](https://github.com/Ghent/capacitarr/commit/99d4504bc28e660688e27b9dcaf2027e663409a1))
- *(auth)* Prevent first-user bootstrap race condition ([de1fb3e](https://github.com/Ghent/capacitarr/commit/de1fb3ee7718922fc058b8642610a5573155eba8))
- *(security)* Hash API keys instead of storing plaintext ([dffe869](https://github.com/Ghent/capacitarr/commit/dffe86984aa1d3a4c22dae94f1affa9ae41b66ae))
- *(security)* Sanitize error responses, add input validation and warnings ([9a646f6](https://github.com/Ghent/capacitarr/commit/9a646f61f906694154dca3dc57e966ad977c9dba))
- *(frontend)* Eliminate all any types with proper TypeScript interfaces ([159b18d](https://github.com/Ghent/capacitarr/commit/159b18dd6af47b9e3e60baaa1c144a0de0128dbe))
- *(frontend)* Remove console.error statements from production code ([75994b8](https://github.com/Ghent/capacitarr/commit/75994b838550dc5a7def271778a9a71f61aa7db3))
- *(css)* Eliminate !important overrides using specificity ([91cbbc0](https://github.com/Ghent/capacitarr/commit/91cbbc0292f28a920b5ffc12d6a0a50e7b402f17))
- *(lint)* Resolve all ESLint warnings and errors ([3203bf8](https://github.com/Ghent/capacitarr/commit/3203bf8af6fb1962257f9d015f8cbef395b5e1c5))
- *(ui)* Update about card with correct repo link, author, and version info ([787e6fd](https://github.com/Ghent/capacitarr/commit/787e6fd8219f648949572a99e1b177b56ee84ed5))
- *(i18n)* Correct langDir path for @nuxtjs/i18n module ([170ef19](https://github.com/Ghent/capacitarr/commit/170ef19bd21bb66f79cdc04dd59a837eb4e08d3f))
- Align help page factor name with rules page slider label ([aa945a8](https://github.com/Ghent/capacitarr/commit/aa945a8c8309fcc246fa19764606f0e75aeadadc))
- Mask integration API keys on edit, preserve on save ([cd103ff](https://github.com/Ghent/capacitarr/commit/cd103ffde563063d496086aa95725f8a5428c833))
- Show masked API key as text in edit modal, clear on focus ([f303671](https://github.com/Ghent/capacitarr/commit/f3036711f8a166e02e04ab93aeffa7f1b289b829))
- Add debug logging to rule creation, ensure new rules enabled by default ([c295adf](https://github.com/Ghent/capacitarr/commit/c295adf0be9e9a7260c6087c9c5ed0e84f0d52bc))
- Ensure rule value sent as string, use debug log level for validation errors ([4f66638](https://github.com/Ghent/capacitarr/commit/4f666388901e967774c058551ae1e165ac0727c7))
- Add log level dropdown, deletion safety status, combobox UX improvements ([0af06c3](https://github.com/Ghent/capacitarr/commit/0af06c3867f86c8bbc1fe1471433b026476422f4))
- Deletion safety button shows correct state-dependent text and variant ([d9b5029](https://github.com/Ghent/capacitarr/commit/d9b5029017608ebd611109036063d9a89a144c6f))
- Improve deletion safety toggle language ([5bdce0d](https://github.com/Ghent/capacitarr/commit/5bdce0d0f95ef98dd8196ba8563bf4f54cbb4ecc))
- Prepend 'Current status:' to deletion safety messages ([57aa2db](https://github.com/Ghent/capacitarr/commit/57aa2dbf3686e0a687dbaaff5c27b085cb148a01))
- Wrap deletion toggle to prevent visual flip before confirmation ([6a49dd6](https://github.com/Ghent/capacitarr/commit/6a49dd6bec75413213db302f2ffd5e35c6496194))
- Deletion toggle uses @update:checked with nextTick for dialog ([91af28b](https://github.com/Ghent/capacitarr/commit/91af28ba14a3a19ee43609c9c1df29544ee755ad))
- *(ui)* Use model-value instead of checked for UiSwitch components ([cf57e30](https://github.com/Ghent/capacitarr/commit/cf57e302908f92caffd334277f51e31581403b22))
- *(rules)* Conflict detection should not flag rules on different fields ([b2957ea](https://github.com/Ghent/capacitarr/commit/b2957ea66138e40903845d1dd0dc1d14310c3457))
- Use 'Series Status' (with space) for score factor display name ([44e87f4](https://github.com/Ghent/capacitarr/commit/44e87f4ffec64aa1d9c3e7bae3e83c3c2300b1ff))
- Revert popover/dropdown to opaque bg, add CSS Ukraine flag to about card ([adc51a8](https://github.com/Ghent/capacitarr/commit/adc51a883f2e324d4f695f56dc68b012cfb81085))
- Notification popup scrolling — replace UiScrollArea with native overflow-y-auto div, simplify Ukraine flag to emoji only ([1014031](https://github.com/Ghent/capacitarr/commit/1014031623c46f510229b69d513cea8b1b51b0b8))
- Use Twemoji SVG for Ukraine flag (bundled locally), replace UiScrollArea with native scroll div for notifications ([19de2dd](https://github.com/Ghent/capacitarr/commit/19de2dd444ddc598c7942c9bc5688565c41529af))
- Deep code audit — fix broken tests, remove dead code, correct docs ([a3dfdfd](https://github.com/Ghent/capacitarr/commit/a3dfdfd8f9e81c70a8d9688b9d5107c31231a610))
- *(css)* Replace oklch relative color syntax with color-mix ([9d21e37](https://github.com/Ghent/capacitarr/commit/9d21e37088ab651b1b817bc3db7f881e9f01ebb1))
- *(ci)* Fix lint job failures and add tag pipeline rules ([89f6904](https://github.com/Ghent/capacitarr/commit/89f69042e8295650013b016c8d1dd16895524041))
- *(ci)* Remove typecheck linter and pin golangci-lint version ([d578bfa](https://github.com/Ghent/capacitarr/commit/d578bfad59b1f0ace62db7d60b78ac471c711065))
- *(ci)* Add git to changelog/goreleaser containers and fix job ordering ([d3ee867](https://github.com/Ghent/capacitarr/commit/d3ee8677338d902877c3e4c696dde9494cd6a4be))
- *(ci)* Use correct package manager for git-cliff (Debian) and add GIT_DEPTH: 0 ([628a73d](https://github.com/Ghent/capacitarr/commit/628a73dd99be6b6b072e335f27ba63b90e4feb07))
- *(ci)* Migrate golangci-lint config to v2 format ([9d9fa4f](https://github.com/Ghent/capacitarr/commit/9d9fa4fcb0a311ceaf4add780da7c103e90c4196))
- Resolve all ESLint errors and warnings ([c5f4d6e](https://github.com/Ghent/capacitarr/commit/c5f4d6e131cd789d724fbf1673a312ad881e5b58))
- Add commit preprocessor to normalize git revert format ([d78b3fa](https://github.com/Ghent/capacitarr/commit/d78b3fa3185c80bd41641f1b8f7aa1a3414b9188))
- *(ci)* Create embed placeholder for Go jobs and normalize revert commits ([6e40594](https://github.com/Ghent/capacitarr/commit/6e40594c195d03be63148845e6eb76779208b387))
- *(backend)* Resolve all 105 golangci-lint v2 issues ([63f4f1e](https://github.com/Ghent/capacitarr/commit/63f4f1eb994a6de24be9ee2d23609cc0fbda16c7))
- Add go:embed placeholder to make check target ([e917633](https://github.com/Ghent/capacitarr/commit/e9176338b94a584cf0302248d75e296e13526022))
- *(ci)* Remove -race flag from Go tests and add vue as dev dependency ([c79462e](https://github.com/Ghent/capacitarr/commit/c79462e2bcb011d76a9402f753c064ad76ebd47b))

### 🛡️ Security

- Add GitLab CI pipeline with lint, test, build, and security stages ([e544a68](https://github.com/Ghent/capacitarr/commit/e544a685de70d98d46eedb0e95168de1e1b1c253))

### ◀️ Revert

- Fix: deletion toggle uses @update:checked with nextTick for dialog ([e530030](https://github.com/Ghent/capacitarr/commit/e53003013051a7ce923ed6024139fc6b15d417e3))
