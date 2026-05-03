# 6. Filesystem Strategy Matrix

The `ExportFilesystem` and `ImportFilesystem` verbs on the `SubstrateAdapter` interface are optional because no universal primitive exists across substrates. VM providers expose disk-level APIs; sandbox providers expose file-level APIs; and some providers offer no export mechanism at all. The latency spread runs from sub-second tar streams to multi-hour disk-image conversions. This chapter catalogs the per-provider reality and derives a three-tier capability taxonomy.

## 6.1 VM Providers: Slow Export (Minutes to Hours)

AWS, GCP, and Azure all export at the disk-image layer. AWS `export-image` converts an AMI to VMDK, VHD, or RAW and stages it in S3 at roughly 30 GB per hour[^1^][^21^]. GCP produces `disk.raw` packaged as a gzipped tar in Cloud Storage via Cloud Build[^23^][^26^]. Azure generates a time-bound SAS URL for VHD download at 60–500 MiB/s depending on disk tier[^2^][^27^]. All three deliver block-level disk images, not filesystem archives.

AWS offers a partial escape hatch through EBS Direct APIs — `GetSnapshotBlock`, `ListSnapshotBlocks`, and `PutSnapshotBlock` — which read snapshot data directly without creating a temporary volume[^4^][^18^]. The API returns raw blocks, not files. Reconstructing a filesystem client-side requires parsing ext4 or xfs metadata structures; no official tool automates this. EBS Direct is a building block for custom tooling, not turnkey export.

DigitalOcean and Hetzner lock snapshots entirely. DigitalOcean snapshots are internal-only; no API endpoint allows downloading the bytes[^5^][^28^]. Hetzner Cloud snapshots are equally locked; the API does not support uploading disk images directly[^6^][^30^]. For both providers the only viable export is through the running instance itself.

## 6.2 Sandbox Providers: Fast Export (Seconds)

Sandbox providers cluster at the opposite end of the latency spectrum. Daytona exposes `fs.upload_file()`, `fs.download_file()`, and batch variants[^7^][^31^]. E2B provides `files.read()` and `files.write()` for individual paths, though directory operations are not supported[^8^][^33^]. Cloudflare's Sandbox SDK offers full file CRUD plus `createBackup()` and `restoreBackup()`, which compress a directory to squashfs and stage it in R2 via presigned URL[^42^][^43^]. All complete in seconds for modest trees.

Modal occupies a category of its own. Its snapshot ecosystem spans filesystem snapshots (diff-based, indefinite), directory snapshots (30-day retention), and memory snapshots (alpha, CRIU-based, sub-second restore)[^10^][^38^][^39^]. Modal Volumes augment this with explicit `.commit()` and `.reload()` semantics[^40^].

Fly Machines present a hybrid case. The root filesystem is ephemeral — it resets from the Docker image on every restart[^11^]. Only attached Volumes, ext4 slices on NVMe drives tied to specific hardware, persist[^35^]. Fly supports daily automatic snapshots, but these are block-level snapshots internal to Fly's infrastructure[^36^]. Exporting a Fly Volume requires `fly ssh console -C 'tar cvz /data'`[^37^], the same tar-over-SSH fallback VM providers use.

Cloudflare Workers are the extreme outlier. The Workers Virtual File System is memory-based: `/bundle` is read-only, `/tmp` is per-request ephemeral, and no state survives across invocations[^12^][^13^]. Workers should be treated as a compute-only substrate with no filesystem export or import.

## 6.3 Self-Hosted: Gold Standard

Self-hosted substrates offer the fastest export paths. `docker export <container>` streams a flat tar archive in seconds[^14^]. The standard pattern — `docker create` to instantiate without starting, `docker export` to capture, `docker rm` to clean up — completes in under ten seconds for images under 1 GB[^44^]. Docker's `import` reconstructs an image from a tarball, providing a symmetric round trip that serves as the reference implementation.

Incus produces `backup.tar.gz` via `incus export <instance>`, with optional `--optimized-storage` for storage-driver-specific formats[^15^]. Optimized exports are faster but can only be restored onto pools using the same driver, a portability tradeoff Docker's flat tar avoids. Firecracker microVMs have no native filesystem export. The workflow chains `docker export` into a loopback ext4 image[^16^]. Firecracker's built-in snapshots capture full microVM state but are resume checkpoints, not portable archives[^45^]. Export is a manual, multi-minute pipeline.

## 6.4 Universal Fallbacks

Where native export is absent, slow, or incomplete, two fallback patterns apply universally.

**tar-over-SSH / tar-over-exec.** On VMs, `ssh user@host "tar czf - --exclude=/proc --exclude=/sys --exclude=/dev /"` streams a compressed filesystem archive directly to standard output[^17^][^46^]. On sandboxes, replace SSH with the provider's `exec()` API. This preserves permissions and extended attributes. The sole prerequisite is a running instance.

**Object-storage staging.** Cloud-init user-data scripts can pull tarballs from S3, GCS, or R2 at boot time[^22^]. The pattern stages an archive in object storage, launches a target with a startup script that downloads and extracts it, then validates checksums. This works across any substrate that supports startup scripts or `exec()` hooks.

## 6.5 Migration Design Implications

The latency stratification has direct consequences for migration architecture. Fast exporters — Docker, Incus, Modal, Cloudflare Sandbox SDK, Daytona — enable live migration with minimal downtime. Slow exporters — AWS, GCP, Azure — force scheduled maintenance windows. Impossible exporters — DigitalOcean, Hetzner, Cloudflare Workers — require rebuilding state from external stores.

Cross-dimensional analysis independently confirms this three-tier taxonomy[^1^][^4^][^5^][^6^][^7^][^10^][^14^]. The `SubstrateAdapter` interface should not treat `ExportFilesystem` as a binary capability. It should advertise latency class — `FastExporter` for sub-minute operations, `SlowExporter` for minute-to-hour operations, and `NoExporter` for substrates where only workarounds exist — so orchestration logic schedules migrations with accurate downtime estimates.

The matrix below consolidates the per-provider findings.

| Provider | Substrate | Export Class | Native API | Format | Latency | Workaround |
|----------|-----------|--------------|------------|--------|---------|------------|
| AWS EC2 | VM | SlowExporter | `export-image` | VMDK/VHD/RAW → S3 | 30–60 min / 30 GB[^1^] | EBS Direct APIs (block-level)[^4^]; tar-over-SSH[^17^] |
| GCP GCE | VM | SlowExporter | `gcloud compute images export` | `disk.raw` in `tar.gz` → GCS | 10–30 min[^23^] | tar-over-SSH; Cloud Build for automation |
| Azure VMs | VM | SlowExporter | SAS URL disk export | VHD | 30 min – hours[^2^] | AzCopy resumable upload[^27^]; Azure VM Run Command |
| DigitalOcean | VM | NoExporter | Snapshots (internal only) | N/A | N/A[^5^][^28^] | rsync-over-SSH[^29^]; tar-over-SSH |
| Hetzner Cloud | VM | NoExporter | Snapshots (internal only) | N/A | N/A[^6^][^30^] | Rescue mode + `dd` over SSH[^6^]; tar-over-SSH |
| Daytona | Sandbox | FastExporter | `fs.upload/download_file` | Raw bytes | Seconds[^7^][^31^] | `exec("tar czf - /workspace")` for full FS |
| E2B | Sandbox | FastExporter | `files.read/write` | Raw bytes | Seconds[^8^] | `commands.run("tar czf - /workspace")` for bulk[^33^] |
| Modal | Sandbox | FastExporter | Filesystem / directory / memory snapshots | Diff-based image | Seconds[^10^][^38^][^39^] | Volume `.commit()` / `.reload()` for shared state[^40^] |
| Fly Machines | Sandbox | NoExporter (rootfs) / SlowExporter (volumes) | Volume snapshots | Block-level | Minutes[^35^][^36^] | `fly ssh console -C 'tar cvz /data'`[^37^] |
| Cloudflare Sandbox | Sandbox | FastExporter | `createBackup()` / `restoreBackup()` | squashfs → R2 | Seconds–minutes[^42^][^43^] | R2 bucket mount for persistent storage |
| Cloudflare Workers | Sandbox | NoExporter | VFS (`/tmp` only) | In-memory | N/A[^12^] | External KV / R2 for state; no FS migration |
| Docker | Self-hosted | FastExporter | `docker export` | Flat tar | Seconds[^14^] | `docker create` → `docker export` → `docker rm`[^44^] |
| Incus | Self-hosted | FastExporter | `incus export` | `tar.gz` | Seconds–minutes[^15^] | `--optimized-storage` for speed; `--compression` for size |
| Firecracker | Self-hosted | SlowExporter | None (manual) | ext4 loopback | Minutes[^16^] | `docker export` → `rootfs.tar` → loopback image[^16^] |

The matrix reveals two dominant clusters. Self-hosted and sandbox providers concentrate in `FastExporter`, with Docker's flat tar setting the speed baseline. VM providers cluster in `SlowExporter`, constrained by disk-image conversion. The `NoExporter` tier is the smallest but operationally critical: DigitalOcean and Hetzner lock snapshots internally, while Cloudflare Workers offer no persistent filesystem. The recommended design is to implement native fast paths where they exist, degrade to tar-over-SSH or tar-over-exec for all other cases, and encode the capability tier in the adapter's `Capabilities()` response.
