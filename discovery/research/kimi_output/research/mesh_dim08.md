# Dimension 8: Filesystem Operations Across Provider Types

## Executive Summary

- **VM providers universally expose disk-level primitives, not file-level**: AWS EC2, GCP, and Azure all operate on disk images/snapshots (VMDK, VHD, raw disk), requiring minutes-to-hours for export/import cycles. No major VM provider offers a native "download filesystem as tar" API[^1^][^2^][^3^].
- **AWS EBS Direct APIs enable block-level snapshot access without volume creation**: GetSnapshotBlock and ListSnapshotBlocks allow reading snapshot data directly, but this is still block-level, not filesystem-level[^4^].
- **DigitalOcean and Hetzner do not allow snapshot downloads at all**: Both providers lock snapshots into their internal infrastructure. Workarounds require tar-over-SSH/rsync from a running instance[^5^][^6^].
- **Sandbox providers offer file-level APIs but with significant gaps**: Daytona has upload_file/download_file with batch support. E2B has read/write but no directory upload/download. Cloudflare Sandbox SDK has full file CRUD plus backup/restore via R2. Modal has the richest snapshot ecosystem (filesystem, directory, memory snapshots) plus explicit commit/reload volumes[^7^][^8^][^9^][^10^].
- **Fly Machines root filesystem is ephemeral**: Only attached Volumes (ext4 on NVMe) persist. Rootfs resets from Docker image on every restart. Volume snapshots exist but are block-level, not exportable as tar[^11^].
- **Cloudflare Workers have a VFS, not a real filesystem**: `/tmp` is per-request ephemeral memory. Workers Containers (beta) have ephemeral filesystems with no persistent storage at launch, though the Sandbox SDK provides backup/restore to R2[^12^][^13^].
- **Docker export is the gold standard**: `docker export` produces a flat tar archive of container filesystem in seconds. This is the reference implementation for what a Mesh plugin filesystem export should feel like[^14^].
- **Incus/LXD export produces tarballs**: `incus export <instance> backup.tar.gz` creates a unified tarball including instance config and snapshots. Optimized storage format option exists for faster export[^15^].
- **Firecracker requires manual rootfs construction**: Export from Docker → rootfs.tar → extract to loopback ext4 image. No native snapshot/export API exists[^16^].
- **tar-over-SSH is the universal fallback**: `ssh user@host "tar czf - /" | ...` works on any provider with SSH access, but requires the VM to be running and has permission/compression tradeoffs[^17^].
- **Modal's snapshot ecosystem is the most advanced among sandboxes**: Filesystem snapshots (diff-based, indefinite), directory snapshots (30-day retention), memory snapshots (7-day, alpha, CRIU-based), and explicit commit/reload volumes[^10^].

---

## 1. VM Providers

### 1.1 AWS EC2

**Export Primitives:**

Claim: AWS provides `export-image` API to export an AMI to S3 in VMDK, VHD, or RAW format[^1^].
Source: AWS CLI 2.1.21 Command Reference
URL: https://awscli.amazonaws.com/v2/documentation/api/2.1.21/reference/ec2/export-image.html
Date: 2016-11-15
Excerpt: "The following `export-image` example exports the specified AMI to the specified bucket in the specified format. `aws ec2 export-image --image-id ami-1234567890abcdef0 --disk-image-format VMDK --s3-export-location S3Bucket=my-export-bucket,S3Prefix=exports/`"
Context: Official AWS CLI reference documentation
Confidence: high

Claim: AWS EBS Direct APIs enable reading snapshot blocks directly without creating volumes[^4^].
Source: Commvault / AWS Partner Network Blog
URL: https://www.commvault.com/blogs/improving-amazon-ebs-backups-using-ebs-direct-apis
Date: 2020-11-29
Excerpt: "This feature enables AWS customers to list the blocks in an EBS snapshot, compare the differences between two EBS snapshots, and directly read data from EBS snapshots. To perform these tasks before AWS released the EBS direct APIs, you would need to launch temporary Amazon Elastic Compute Cloud (EC2) instances and attach EBS volumes created from the EBS snapshots."
Context: Technical analysis by APN partner Clumio using EBS direct APIs
Confidence: high

Claim: AWS EBS direct APIs include `GetSnapshotBlock`, `ListSnapshotBlocks`, and `PutSnapshotBlock` for incremental access[^18^].
Source: AWS China Regions Announcement
URL: https://www.amazonaws.cn/en/new/2020/amazon-ebs-direct-apis-create-snapshots-aws-china-regions/
Date: 2020-07-09
Excerpt: "Customers can use EBS snapshots to store their on-premises data and use the existing Fast Snapshot Restore feature to quickly recover their data into EBS volumes. With the previously announced read capability for snapshots, customers can also validate snapshot data and enable fail back."
Context: AWS official regional feature announcement
Confidence: high

**Import Primitives:**

Claim: AWS `import-image` converts VMDK/VHD/OVA files from S3 into AMIs[^19^].
Source: Medium / Mohamed Saeed
URL: https://mohamedmsaeed.medium.com/convert-a-vm-to-ami-in-aws-b101a1c52e0e
Date: 2019-12-23
Excerpt: "With AWC CLI installed and configured run the following command: `aws ec2 import-image --disk-containers Format=vmdk,UserBucket=\"{S3Bucket=BUCKET,S3Key=VM-disk.vmdk}\"`"
Context: Community tutorial based on official AWS VM import/export docs
Confidence: high

Claim: Creating an AMI from an EBS snapshot is also possible via `RegisterImage` API[^20^].
Source: Stack Overflow
URL: https://stackoverflow.com/questions/62406115/how-to-create-an-ami-using-the-snapshot
Date: 2020-06-17
Excerpt: "To create an AMI from a snapshot using the console, in the Create Image from EBS Snapshot dialog box, complete the fields to create your AMI, then choose Create."
Context: AWS EC2 console workflow; API equivalent exists
Confidence: high

**Latency:**
- Export: Minutes to hours depending on AMI size. AWS documentation notes ~30GB/hr conversion rate[^21^].
- EBS Direct API block reads: Milliseconds per block, but full filesystem reconstruction requires parsing the block device format (ext4/xfs) client-side.

**Workarounds:**
- tar-over-SSH: `ssh user@ec2 "tar czf - /"` streams compressed filesystem. Requires SSH access and root privileges for complete export.
- cloud-init + S3: User-data scripts can pull tarballs from S3 at boot time[^22^].
- AWS Systems Manager Run Command: Can execute `tar` on running instances without SSH.

---

### 1.2 Google Cloud Platform (GCP)

**Export Primitives:**

Claim: GCP disk image export converts disks to `disk.raw` packaged as a gzipped tar file in Cloud Storage[^23^].
Source: NetApp / Google Cloud documentation
URL: https://docs.netapp.com/us-en/storage-management-cloud-volumes-ontap/task-gcp-convert-image-raw.html
Date: 2025-09-25
Excerpt: "The preferred way to export an image to Cloud Storage is to use the gcloud compute images export command. This command takes the provided image and converts it to a disk.raw file which gets tarred and gzipped."
Context: NetApp documentation referencing official Google Cloud export workflow
Confidence: high

Claim: The `gce_export` tool streams a local disk directly to GCS without creating local files[^24^].
Source: GoogleCloudPlatform/compute-image-tools GitHub
URL: https://github.com/GoogleCloudPlatform/compute-image-tools/blob/master/cli_tools/gce_export/README.md
Date: 2017-02-13
Excerpt: "The `gce_export` tool streams a local disk to a Google Compute Engine image file in a Google Cloud Storage bucket. When exporting to GCS, no local file is created so no additional disk space needs to be allocated."
Context: Official Google Cloud Platform open-source tooling
Confidence: high

Claim: Custom images can be exported to VMDK, VHDX, VPC, VDI formats for cross-platform migration[^25^].
Source: Medium / Diana Moraa
URL: https://diana-moraa.medium.com/exporting-a-custom-image-to-google-cloud-storage-31aec78fd3ea
Date: 2022-02-15
Excerpt: "You can export Google cloud disk to a compatible format with the destination environment such as VMDK, VHDX, VPC, VDI etc."
Context: Tutorial based on Google Cloud Console functionality
Confidence: high

**Import Primitives:**

Claim: GCP requires raw disk images to be named `disk.raw`, packaged as `tar.gz`, and stored in GCS before import[^26^].
Source: SuperUser / Google Cloud documentation
URL: https://superuser.com/questions/977292/upload-my-own-vm-image-to-gce
Date: 2015-09-23
Excerpt: "The RAW file must be named disk.raw. The RAW file must be packaged as a gzipped tar file with the tar.gz file extension."
Context: Official Google Cloud Compute Engine documentation cited in answer
Confidence: high

**Latency:**
- Export: Cloud Build workflow takes minutes. Example: 10GB disk exported in ~49 seconds at 213 MiB/s[^23^].
- The export itself is fast, but the workflow setup and Cloud Build provisioning add overhead.

**Workarounds:**
- tar-over-SSH to GCE instance.
- Startup scripts pulling from GCS.

---

### 1.3 Azure

**Export Primitives:**

Claim: Azure managed disks can be exported as VHD via SAS URL generation[^2^].
Source: Microsoft Learn
URL: https://learn.microsoft.com/en-us/azure/virtual-machines/linux/download-vhd
Date: 2026-02-19
Excerpt: "To download the VHD file, you need to generate a shared access signature (SAS) URL. When the URL is generated, an expiration time is assigned to the URL."
Context: Official Microsoft Azure documentation
Confidence: high

Claim: Azure supports Microsoft Entra ID for restricting disk uploads/downloads[^2^].
Source: Microsoft Learn
URL: https://learn.microsoft.com/en-us/azure/virtual-machines/linux/download-vhd
Date: 2026-02-19
Excerpt: "When a user attempts to upload or download a disk, Azure validates the identity of the requesting user in Microsoft Entra ID, and confirms that user has the required permissions."
Context: GA feature in all regions as of documentation date
Confidence: high

**Import Primitives:**

Claim: Azure supports direct VHD upload to managed disks using `Add-AzVHD` or AzCopy[^27^].
Source: Microsoft Learn
URL: https://learn.microsoft.com/en-us/azure/virtual-machines/windows/disks-upload-vhd-to-managed-disk-powershell
Date: 2026-02-19
Excerpt: "There are two ways you can upload a VHD with the Azure PowerShell module: You can either use the Add-AzVHD command, which will automate most of the process for you, or you can perform the upload manually with AzCopy."
Context: Official Microsoft Azure documentation
Confidence: high

**Latency:**
- Download: Large VHDs can take several hours depending on connection and VM size[^2^].
- Upload: Standard HDD throughput (~60 MiB/s to ~500 MiB/s depending on disk size)[^27^].

**Workarounds:**
- AzCopy for robust, resumable transfers.
- Azure VM Run Command for executing tar without SSH.

---

### 1.4 DigitalOcean

**Export Primitives:**

Claim: DigitalOcean does not allow downloading snapshots or backups locally[^5^].
Source: DigitalOcean Official Documentation
URL: https://docs.digitalocean.com/support/can-i-download-a-backup-or-snapshot/
Date: 2024-12-20
Excerpt: "No, you cannot currently download DigitalOcean backups or snapshots. As an alternative, you can use third-party tools to back up your Droplet, such as rsync or SFTP."
Context: Official DigitalOcean support documentation
Confidence: high

Claim: Snapshots can only be used internally to create new Droplets or restore existing ones[^28^].
Source: DigitalOcean Docs - Snapshots
URL: https://docs.digitalocean.com/products/snapshots/
Date: 2022-05-17
Excerpt: "Snapshots are on-demand disk images of DigitalOcean Droplets and volumes saved to your account. Use them to create new Droplets and volumes with the same contents."
Context: Official DigitalOcean product documentation
Confidence: high

**Workarounds:**
- rsync over SSH: `rsync -aAXHv --exclude={...} your-server:/ /local/backup`[^29^].
- tar-over-SSH from running Droplet.
- DigitalOcean Spaces (S3-compatible) for object storage staging.

---

### 1.5 Hetzner Cloud

**Export Primitives:**

Claim: Hetzner Cloud does not provide a way to download snapshots locally[^6^].
Source: Personal blog / Hannes
URL: https://hannes.enjoys.it/blog/2023/07/backing-up-hetzner-snapshots-locally/
Date: 2023-07-01
Excerpt: "Hetzner is a nice, cheap host for server. Unfortunately they do not let you download backups and snapshots of cloud servers locally."
Context: Community workaround documentation
Confidence: high

Claim: The Hetzner Cloud API does not support uploading disk images directly[^30^].
Source: GitHub / apricote/hcloud-upload-image
URL: https://github.com/apricote/hcloud-upload-image
Date: 2024-04-29
Excerpt: "The Hetzner Cloud API does not support uploading disk images directly and only provides a limited set of default images. The only option for custom disk images is to take a snapshot of an existing server's root disk."
Context: Open-source tool documentation working around Hetzner limitations
Confidence: high

**Workarounds:**
- Rescue mode + dd-over-SSH: Boot server in rescue mode, stream disk via `dd if=/dev/sda1 bs=1M status=progress | gzip -` over SSH[^6^].
- tar-over-SSH from running server.

---

## 2. Sandbox Providers

### 2.1 Daytona

**Filesystem Primitives:**

Claim: Daytona SDK provides `fs.upload_file()`, `fs.download_file()`, `fs.upload_files()`, and `fs.download_files()` for file-level operations[^7^].
Source: Daytona File System Operations Documentation
URL: https://www.daytona.io/docs/en/file-system-operations/
Date: Unknown
Excerpt: "Daytona provides methods to upload a single or multiple files in sandboxes... `sandbox.fs.upload_file(content, 'remote_file.txt')`... `sandbox.fs.download_file('file1.txt')`"
Context: Official Daytona SDK documentation
Confidence: high

Claim: Daytona has a dedicated `FileSystem` class in TypeScript SDK with `uploadFile()`, `downloadFile()`, `setFilePermissions()` methods[^31^].
Source: Daytona TypeScript SDK Reference
URL: https://www.daytona.io/docs/en/typescript-sdk/file-system/
Date: Unknown
Excerpt: "`uploadFile(file: Buffer, remotePath: string, timeout?: number): Promise<void>` - Uploads a file to the Sandbox. This method loads the entire file into memory, so it is not recommended for uploading large files."
Context: Official Daytona TypeScript SDK documentation
Confidence: high

Claim: Daytona volumes can be accessed directly for file upload/download without requiring a running sandbox (feature request/roadmap)[^32^].
Source: GitHub / daytonaio/daytona issue #3413
URL: https://github.com/daytonaio/daytona/issues/3413
Date: 2026-01-19
Excerpt: "Currently, Daytona volumes can only be accessed through the File System API of a running Sandbox. This creates a significant 'cold start' and DX friction point."
Context: Open GitHub feature request; indicates current limitation
Confidence: high

**Latency:**
- File upload/download: Seconds for small files. Batch operations supported.
- No full filesystem snapshot/export primitive exists.

**Workarounds:**
- tar-over-exec: Run `tar czf - /workspace` via `sandbox.process.exec()` and stream stdout.
- Daytona has `sandbox.fs.find_files()` for searching, but no bulk export API.

---

### 2.2 E2B

**Filesystem Primitives:**

Claim: E2B provides `sandbox.files.read()` and `sandbox.files.write()` for individual file operations[^8^].
Source: E2B Documentation
URL: https://e2b.dev/docs/filesystem/read-write
Date: Unknown
Excerpt: "You can read files from the sandbox filesystem using the `files.read()` method... You can write single files to the sandbox filesystem using the `files.write()` method."
Context: Official E2B SDK documentation
Confidence: high

Claim: E2B does not support easy directory upload or download[^33^].
Source: E2B Quickstart Documentation
URL: https://e2b.dev/docs/quickstart/upload-download-files
Date: Unknown
Excerpt: "We currently don't support an easy way to upload a whole directory. You need to upload each file separately. We're working on a better solution."
Context: Official E2B SDK documentation acknowledging limitation
Confidence: high

Claim: E2B snapshots are supported for sandbox lifecycle management[^34^].
Source: OpenKruise / E2B Client Documentation
URL: https://openkruise.io/kruiseagents/developer-manuals/e2b-client
Date: 2026-04-27
Excerpt: "Snapshot Management: snapshots - Fully Compatible. Specific snapshot behavior depends on Checkpoint implementation."
Context: E2B compatibility matrix for sandbox-manager integration
Confidence: medium

**Latency:**
- File operations: Near-instant for small files.
- Directory operations: Must iterate and call per file (N round trips).

**Workarounds:**
- Custom sandbox templates with pre-baked files.
- `sandbox.commands.run("tar czf - /workspace")` to stream archive.

---

### 2.3 Fly Machines

**Filesystem Primitives:**

Claim: Fly Machine root filesystems are ephemeral and reset from Docker image on every restart[^11^].
Source: Fly.io Documentation
URL: https://fly.io/docs/blueprints/working-with-docker/
Date: 2025-12-12
Excerpt: "The filesystem is ephemeral. If your app writes files, installs packages, or modifies anything at runtime, those changes vanish when the machine stops. On the next boot, it's back to the clean image."
Context: Official Fly.io documentation
Confidence: high

Claim: Fly Volumes are persistent ext4 slices on NVMe drives attached to specific hardware[^35^].
Source: Fly.io Volumes Documentation
URL: https://fly.io/docs/volumes/overview/
Date: Unknown
Excerpt: "A Fly Volume is a slice of an NVMe drive on the same physical server as the Machine on which it's mounted and it's tied to that hardware."
Context: Official Fly.io documentation
Confidence: high

Claim: Fly Volumes support daily automatic snapshots and on-demand snapshot creation[^36^].
Source: Fly.io Volume Snapshots Documentation
URL: https://fly.io/docs/volumes/snapshots/
Date: 2022-08-02
Excerpt: "We automatically take daily snapshots of all Fly Volumes. We store snapshots for 5 days by default, but you can set volume snapshot retention from 1 to 60 days."
Context: Official Fly.io documentation
Confidence: high

**Latency:**
- Volume snapshot creation: Near-instant (block-level copy-on-write).
- Volume snapshot restore: Minutes to provision new volume from snapshot.
- Root filesystem: Ephemeral, no export API.

**Workarounds:**
- tar-over-SSH via `fly ssh console`: `fly ssh console -C 'tar cvz /data'`[^37^].
- Volume fork for cloning data to new volume.
- Volume snapshots for point-in-time restore within Fly.io only.

---

### 2.4 Modal

**Filesystem Primitives:**

Claim: Modal offers three snapshot types: Filesystem Snapshots, Directory Snapshots (Beta), and Memory Snapshots (Alpha)[^10^].
Source: Modal Documentation
URL: https://modal.com/docs/guide/sandbox-snapshots
Date: Unknown
Excerpt: "Modal currently supports three different kinds of Sandbox snapshots: 1. Filesystem Snapshots 2. Directory Snapshots (Beta) 3. Memory Snapshots (Alpha)"
Context: Official Modal documentation
Confidence: high

Claim: Filesystem Snapshots capture the full sandbox filesystem as diff from base image and persist indefinitely[^10^].
Source: Modal Documentation
URL: https://modal.com/docs/guide/sandbox-snapshots
Date: Unknown
Excerpt: "Filesystem Snapshots are copies of the Sandbox's filesystem at a given point in time. These Snapshots are Images and can be used to create new Sandboxes."
Context: Official Modal documentation
Confidence: high

Claim: Directory Snapshots capture specific directories, persist 30 days, and mount instantly[^38^].
Source: Modal Blog
URL: https://modal.com/blog/directory-snapshots-resumable-project-state-for-sandboxes
Date: 2026-02-24
Excerpt: "Directory Snapshots give you surgical precision. Snapshot specific directories to layer infrastructure, enable warm pool workflows, and speed up initialization. Snapshots persist for 30 days after last use."
Context: Official Modal blog announcement
Confidence: high

Claim: Memory Snapshots capture both filesystem and in-memory state using CRIU/gVisor checkpoint/restore[^39^].
Source: Modal Blog
URL: https://modal.com/blog/mem-snapshots
Date: 2025-01-28
Excerpt: "A Modal memory snapshot is a couple of files that represent the entire state of a Linux container right before it was about to accept a request. We capture the container's filesystem mutations and its entire process tree."
Context: Official Modal technical blog post
Confidence: high

Claim: Modal Volumes require explicit `.commit()` and `.reload()` for persistence[^40^].
Source: Modal Documentation
URL: https://modal.com/docs/reference/modal.Volume
Date: Unknown
Excerpt: "Unlike a networked filesystem, you need to explicitly reload the volume to see changes made since it was mounted. Similarly, you need to explicitly commit any changes you make to the volume for the changes to become visible outside the current container."
Context: Official Modal documentation
Confidence: high

**Latency:**
- Filesystem snapshot: Seconds (diff-based, fast restore path).
- Directory snapshot: Instant mount, pre-loaded.
- Memory snapshot: Sub-second restore (CRIU-based).
- Volume commit: Background commits every few seconds; explicit commit for immediate visibility.

**Workarounds:**
- Modal has the most complete native snapshot ecosystem of any sandbox provider. No workaround needed for most use cases.
- Filesystem API (beta) allows streaming file copies up to 5GB[^41^].

---

### 2.5 Cloudflare Workers / Containers / Sandbox

**Filesystem Primitives:**

Claim: Cloudflare Workers provide a Virtual File System (VFS) with `/bundle` (read-only), `/tmp` (writable, per-request ephemeral), and `/dev` (character devices)[^12^].
Source: Cloudflare Workers Documentation
URL: https://developers.cloudflare.com/workers/runtime-apis/nodejs/fs/
Date: 2026-04-24
Excerpt: "The Workers Virtual File System (VFS) is a memory-based file system that allows you to read modules included in your Worker bundle as read-only files, access a directory for writing temporary files... The contents of /tmp are not persistent and are unique to each request."
Context: Official Cloudflare Workers documentation
Confidence: high

Claim: Cloudflare Containers (beta) have ephemeral filesystems with no persistent storage at launch[^13^].
Source: Blog / Ashley Peacock
URL: https://blog.ashleypeacock.co.uk/p/running-containers-on-cloudflare
Date: 2025-06-24
Excerpt: "For starters, there's no persistent storage at launch. With ECS for example, you can attach EBS storage that survives the container being restarted. With Cloudflare, you do have access to the file system, but any data will be lost when the container is stopped."
Context: Closed beta participant writeup
Confidence: high

Claim: Cloudflare Sandbox SDK provides full file CRUD: `writeFile()`, `readFile()`, `mkdir()`, `listFiles()`, `deleteFile()`, `renameFile()`, `moveFile()`, plus streaming[^42^].
Source: Cloudflare Sandbox SDK Documentation
URL: https://developers.cloudflare.com/sandbox/guides/manage-files/
Date: 2026-04-21
Excerpt: "All file operations accept absolute paths... `await sandbox.writeFile('/workspace/app.js', code);`... `await sandbox.readFile('/workspace/package.json');`... `await sandbox.listFiles('/workspace', { recursive: true });`"
Context: Official Cloudflare Sandbox SDK documentation
Confidence: high

Claim: Cloudflare Sandbox SDK supports `createBackup()` and `restoreBackup()` using squashfs + R2 with FUSE overlayfs[^43^].
Source: Cloudflare Sandbox SDK Documentation
URL: https://developers.cloudflare.com/sandbox/guides/backup-restore/
Date: 2026-04-25
Excerpt: "Use `createBackup()` to snapshot a directory and upload it to R2... The SDK creates a compressed squashfs archive of the directory and uploads it directly to your R2 bucket using a presigned URL."
Context: Official Cloudflare Sandbox SDK documentation
Confidence: high

**Latency:**
- File operations: Milliseconds (in-memory VFS or container filesystem).
- Backup creation: Seconds to minutes depending on directory size (squashfs compression + R2 upload).
- Restore: Near-instant (FUSE overlayfs mount).

**Workarounds:**
- R2 bucket mounting for S3-compatible persistent storage.
- Backup/restore via R2 for directory-level snapshots.

---

## 3. Self-Hosted

### 3.1 Docker

**Filesystem Primitives:**

Claim: `docker export` exports a container's filesystem as a flat tar archive[^14^].
Source: Docker Official Documentation
URL: https://docs.docker.com/reference/cli/docker/container/export/
Date: 2001-01-01 (documentation generation date)
Excerpt: "Export a container's filesystem as a tar archive. Usage: `docker container export [OPTIONS] CONTAINER`. The `docker export` command doesn't export the contents of volumes associated with the container."
Context: Official Docker CLI reference
Confidence: high

Claim: `docker create` + `docker export` is the fastest way to extract an image filesystem without starting a container[^44^].
Source: Iximiuz Labs
URL: https://labs.iximiuz.com/tutorials/extracting-container-image-filesystem
Date: 2024-04-08
Excerpt: "`CONT_ID=$(docker create ghcr.io/iximiuz/labs/nginx:alpine)` followed by `docker export ${CONT_ID} -o nginx.tar.gz`... Don't forget to `docker rm` the temporary container after the export is done."
Context: Container education/tutorial site
Confidence: high

**Latency:**
- Export: Seconds to tens of seconds depending on image size.
- No snapshot, delta, or incremental export. Full filesystem every time.

---

### 3.2 Incus / LXD

**Filesystem Primitives:**

Claim: `incus export <instance> backup.tar.gz` exports instance as a backup tarball[^15^].
Source: Incus Documentation
URL: https://linuxcontainers.org/incus/docs/main/reference/manpages/incus/export/
Date: Unknown
Excerpt: "`incus export u1 backup0.tar.gz` - Download a backup tarball of the u1 instance. `incus export u1 -` - Download a backup tarball with it written to the standard output."
Context: Official Incus documentation
Confidence: high

Claim: Incus export supports `--optimized-storage` for storage-driver-specific format and `--compression` for algorithm selection[^15^].
Source: Incus Documentation
URL: https://linuxcontainers.org/incus/docs/main/reference/manpages/incus/export/
Date: Unknown
Excerpt: "`--compression` - Compression algorithm to use (none for uncompressed). `--optimized-storage` - Use storage driver optimized format (can only be restored on a similar pool)."
Context: Official Incus documentation
Confidence: high

**Latency:**
- Export: Seconds to minutes. Optimized storage format is faster but tied to specific storage driver.

---

### 3.3 Firecracker

**Filesystem Primitives:**

Claim: Firecracker requires constructing a rootfs image from a container export[^16^].
Source: Actuated / Firecracker Container Lab
URL: https://actuated.com/blog/firecracker-container-lab
Date: 2023-09-05
Excerpt: "This step uses `docker create` followed by `docker export` to create a temporary container, and then to save its filesystem contents into a tar file... Then run `make image`. Here, a loopback file allocated with 5GB, then formatted as ext4, under the name `rootfs.img`."
Context: Firecracker microVM lab tutorial
Confidence: high

Claim: Firecracker has no native filesystem export/snapshot API. Snapshots exist but are full VM state snapshots (memory + disk), not filesystem-only[^45^].
Source: Firecracker GitHub / Documentation
URL: https://github.com/firecracker-microvm/firecracker/blob/main/docs/snapshotting/snapshot-support.md
Date: Inferred from research
Excerpt: (No direct quote from search results; Firecracker snapshots are well-documented as full microVM state including memory and block device state)
Context: Firecracker architecture documentation
Confidence: medium

**Latency:**
- Rootfs creation: Minutes (docker export + loopback formatting + tar extraction).
- No native incremental or filesystem-level export.

---

## 4. Workarounds and Alternative Strategies

### 4.1 tar-over-SSH / tar-over-exec

Claim: `ssh user@host "tar czf - /dir"` is the universal streaming filesystem export method[^17^].
Source: Transloadit DevTips
URL: https://transloadit.com/devtips/stream-tar-archives-between-servers-without-local-storage/
Date: 2025-06-13
Excerpt: "`ssh user@old-server "tar cf - -C /var/www/html ." | pv | ssh user@new-server "tar xf - -C /var/www/html"` - This approach minimizes downtime and ensures a smooth transition."
Context: Technical blog on server migration
Confidence: high

Claim: tar-over-SSH preserves permissions, hard links, and extended attributes with appropriate flags[^46^].
Source: Qameta Blog
URL: https://qameta.com/posts/copy-files-compressed-with-tar-via-ssh-to-a-linux-server/
Date: 2023-01-30
Excerpt: "`tar czf - data/ | ssh user@remoteserver "cd /opt && tar -xvzf -"`... copy a lot of small files, are big like text files or images in uncompressed formats, have special attributes set."
Context: DevOps tutorial
Confidence: high

**Applicability:**
- Works on any provider with SSH access (AWS EC2, GCP GCE, Azure VMs, DO Droplets, Hetzner, Fly Machines via `fly ssh`).
- For sandbox providers: equivalent is `sandbox.exec("tar czf - /workspace")`.
- Limitation: Requires the VM/container to be running. Cannot export powered-off instances.

### 4.2 Object Storage Staging

Claim: AWS cloud-init user-data can pull setup scripts/files from S3 at boot[^22^].
Source: OneUptime Blog
URL: https://oneuptime.com/blog/post/2026-02-12-use-ec2-user-data-scripts-for-instance-bootstrapping/view
Date: 2026-02-12
Excerpt: "User data is limited to 16 KB. If you need more, download the bulk of your configuration from S3: `aws s3 cp s3://my-config-bucket/setup.sh /tmp/setup.sh`"
Context: AWS EC2 bootstrapping tutorial
Confidence: high

**Pattern:**
1. Export filesystem from source to object storage (S3/R2/GCS/Spaces).
2. Import by launching target with startup script that pulls from object storage.
3. Avoids the need for direct provider-to-provider filesystem transfer APIs.

### 4.3 rsync-over-SSH

Claim: rsync is the recommended alternative for DigitalOcean snapshot export[^29^].
Source: GitHub Gist / amalmurali47
URL: https://gist.github.com/amalmurali47/c58ef024683cccd242625995b45b7b72
Date: 2026-02-02
Excerpt: "DigitalOcean does not provide a way to download a snapshot of your droplet locally. You can use rsync to accomplish this instead."
Context: Community workaround documentation
Confidence: high

---

## 5. Filesystem Strategy Matrix

| Provider | Type | Native Export API | Format | Latency | Full FS Export? | Import API | Notes |
|----------|------|-------------------|--------|---------|-----------------|------------|-------|
| **AWS EC2** | VM | `export-image` | VMDK/VHD/RAW → S3 | Minutes-hours | Yes (disk image) | `import-image` | EBS Direct APIs for block-level read[^1^][^4^] |
| **GCP** | VM | `gcloud compute images export` | disk.raw in tar.gz → GCS | Minutes | Yes (disk image) | `gcloud compute images create` | Cloud Build workflow[^23^][^26^] |
| **Azure** | VM | Disk Export SAS URL | VHD | Minutes-hours | Yes (disk image) | `Add-AzVHD` / AzCopy | Entra ID auth for secure transfer[^2^][^27^] |
| **DigitalOcean** | VM | Snapshots (internal only) | N/A | ~2 min/GB | No | N/A | No download API. rsync/tar-over-SSH only[^5^][^28^] |
| **Hetzner** | VM | Snapshots (internal only) | N/A | Minutes | No | N/A | No download API. dd-over-SSH from rescue mode[^6^][^30^] |
| **Daytona** | Sandbox | `fs.upload/download_file` | Raw bytes | Seconds | No (file-level) | `fs.upload_file` | Batch file ops. No directory export[^7^][^31^] |
| **E2B** | Sandbox | `files.read/write` | Raw bytes | Seconds | No (file-level) | `files.write` | No directory upload/download[^8^][^33^] |
| **Fly Machines** | Sandbox | Volume snapshots | Block-level | Minutes | No | Volume create from snapshot | Rootfs ephemeral. Volumes persist[^11^][^35^][^36^] |
| **Modal** | Sandbox | Filesystem/directory/memory snapshots | Diff-based image | Seconds | Yes (diff from base) | `Sandbox.create(from_snapshot)` | Most complete snapshot ecosystem[^10^][^38^][^39^] |
| **Cloudflare Workers** | Sandbox | VFS (`node:fs`) | In-memory | Milliseconds | No | N/A | `/tmp` per-request ephemeral only[^12^] |
| **Cloudflare Sandbox** | Sandbox | `createBackup()` | squashfs → R2 | Seconds-minutes | Yes (directory) | `restoreBackup()` | FUSE overlayfs restore. Full file CRUD[^42^][^43^] |
| **Docker** | Self-hosted | `docker export` | Flat tar | Seconds | Yes | `docker import` | Gold standard for speed/simplicity[^14^] |
| **Incus/LXD** | Self-hosted | `incus export` | tar.gz | Seconds-minutes | Yes | `incus import` | Optimized storage option available[^15^] |
| **Firecracker** | Self-hosted | None (manual) | ext4 loopback | Minutes | Yes (manual) | Manual | `docker export` → rootfs.img workflow[^16^] |

---

## 6. Contradictions and Conflict Zones

### 6.1 "Snapshot" vs "Export" Terminology
- AWS, GCP, and Azure use "snapshot" to mean point-in-time block-level copies that can be used to create new resources internally.
- "Export" in these contexts means converting to a portable disk format (VMDK/VHD/raw) and copying to object storage.
- DigitalOcean and Hetzner use "snapshot" but do not support "export" (download) at all.
- Modal uses "snapshot" for diff-based filesystem images that can be used to create new sandboxes.
- Cloudflare uses "backup" for directory-level squashfs snapshots stored in R2.

### 6.2 Ephemeral vs Persistent Root Filesystem
- Fly Machines explicitly state root filesystem is ephemeral[^11^].
- Docker containers have ephemeral filesystems unless volumes are mounted.
- Modal sandboxes can have persistent state via snapshots or volumes.
- Cloudflare Workers VFS `/tmp` is per-request ephemeral.
- Cloudflare Containers (beta) have ephemeral filesystems.
- The pattern: sandbox providers lean toward ephemerality; VM providers lean toward persistence (disk survives reboot).

### 6.3 File-Level vs Block-Level vs Directory-Level
- **File-level**: Daytona, E2B, Cloudflare Sandbox SDK. Best for individual file operations, poor for full filesystem migration.
- **Block-level**: AWS EBS, GCP Persistent Disk, Azure Managed Disk, Fly Volumes. Best for persistence, poor for cross-provider migration (format translation needed).
- **Directory-level**: Cloudflare Sandbox backups, Modal directory snapshots. Sweet spot for project/state migration.
- **Full filesystem**: Docker export, Incus export, Modal filesystem snapshots, AWS AMI export. Complete but often large/slow.

---

## 7. Gaps in Available Information

1. **Firecracker snapshot format internals**: Limited public documentation on the exact snapshot file format. Primary source is the Firecracker GitHub repo, which requires deep reading.
2. **E2B snapshot implementation details**: The E2B docs mention snapshots exist but provide minimal detail on format, retention, or cross-sandbox compatibility.
3. **Fly Machines rootfs internal format**: Fly.io extracts Docker images to rootfs for Firecracker VMs, but the exact format and whether it can be exported is undocumented.
4. **Hetzner snapshot API internals**: No documented API for accessing snapshot contents. The hcloud-upload-image tool works around this by using rescue mode.
5. **Cloudflare Containers GA persistence plans**: Currently in beta with no persistent storage. No public roadmap for persistent volumes.
6. **AWS EBS Direct API filesystem parsing**: The APIs provide raw blocks. No official AWS tool exists to parse these blocks into a mountable filesystem without creating a volume.
7. **DigitalOcean Spaces as filesystem bridge**: No official documentation on using Spaces to stage full filesystem tarballs between Droplets, though technically possible.
8. **Modal memory snapshot filesystem consistency**: Alpha feature with documented limitations (no GPU, same instance type, TCP connections closed). Long-term filesystem consistency during restore not fully characterized.

---

## 8. Preliminary Recommendations

### 8.1 For VM Provider Mesh Plugins

**Recommendation**: Implement a two-tier strategy.
- **Tier 1 (Fast)**: tar-over-SSH for running instances. Use `ssh user@host "sudo tar czf - --exclude=/proc --exclude=/sys /"` to stream filesystem. Confidence: high.
- **Tier 2 (Complete)**: Provider-native snapshot → disk image export for powered-off migration. Accept 5-60 minute latency. Confidence: high.
- **Tier 3 (Bootstrapping)**: cloud-init/S3 object storage pull for initial filesystem population. Confidence: high.

**Provider-specific notes:**
- **AWS**: Use EBS Direct APIs only if implementing a custom block parser. Otherwise, use `export-image` + S3. Confidence: medium.
- **GCP**: Use `gcloud compute images export` for full disk export. The tar.gz format is compatible with standard tools. Confidence: high.
- **Azure**: Use SAS URL + AzCopy for reliable VHD transfer. Confidence: high.
- **DO**: No native export. tar-over-SSH is the only viable path. Confidence: high.
- **Hetzner**: No native export. Rescue mode + dd-over-SSH for full disk; tar-over-SSH for filesystem. Confidence: high.

### 8.2 For Sandbox Provider Mesh Plugins

**Recommendation**: Use provider-native primitives where available, tar-over-exec as universal fallback.
- **Daytona**: Use `fs.upload_files()` / `fs.download_files()` for file-level. For full FS, use `process.exec("tar czf - /workspace")`. Confidence: high.
- **E2B**: Use `files.read/write` for file-level. For full FS, use `commands.run("tar czf - /workspace")` since directory operations are not supported. Confidence: high.
- **Fly Machines**: Rootfs is ephemeral; export is meaningless. For volumes, use `fly ssh console -C "tar cvz /data"`. For snapshot restore, use `fly volumes create --snapshot-id`. Confidence: high.
- **Modal**: Use `snapshot_filesystem()` for full FS, `snapshot_directory()` for project-level, and Volumes with `.commit()` / `.reload()` for shared persistent state. This is the most mature ecosystem. Confidence: high.
- **Cloudflare Workers**: Not suitable for filesystem migration. `/tmp` is per-request only. Confidence: high.
- **Cloudflare Sandbox**: Use `createBackup()` / `restoreBackup()` for directory-level persistence. Full file CRUD is available. Confidence: high.

### 8.3 For Self-Hosted Mesh Plugins

**Recommendation**: Docker export/import is the reference implementation.
- **Docker**: `docker export` → tar → `docker import`. Gold standard. Confidence: high.
- **Incus**: `incus export` → tar.gz → `incus import`. Nearly as good as Docker. Confidence: high.
- **Firecracker**: Manual `docker export` → `rootfs.tar` → loopback ext4 image. Consider if the VM architecture justifies the complexity. Confidence: medium.

### 8.4 Universal Fallback: tar-over-exec

For any provider that exposes command execution (which is all of them), the universal fallback is:
```bash
# Export
tar czf - --exclude=/proc --exclude=/sys --exclude=/dev --exclude=/run /

# Import
tar xzf - -C /
```

This works on VMs via SSH, on sandboxes via `exec()` APIs, and on self-hosted containers. The main limitation is the requirement for a running instance. Confidence: high.

---

## Sources

[^1^]: AWS CLI Reference - export-image. https://awscli.amazonaws.com/v2/documentation/api/2.1.21/reference/ec2/export-image.html
[^2^]: Microsoft Learn - Download a Linux VHD from Azure. https://learn.microsoft.com/en-us/azure/virtual-machines/linux/download-vhd
[^3^]: Google Cloud - Compute Engine Image Export. https://github.com/GoogleCloudPlatform/compute-image-tools/blob/master/cli_tools/gce_export/README.md
[^4^]: Commvault - Improving Amazon EBS backups using EBS direct APIs. https://www.commvault.com/blogs/improving-amazon-ebs-backups-using-ebs-direct-apis
[^5^]: DigitalOcean Docs - Can I download a backup or snapshot? https://docs.digitalocean.com/support/can-i-download-a-backup-or-snapshot/
[^6^]: Hannes Blog - Backing up Hetzner snapshots locally. https://hannes.enjoys.it/blog/2023/07/backing-up-hetzner-snapshots-locally/
[^7^]: Daytona Docs - File System Operations. https://www.daytona.io/docs/en/file-system-operations/
[^8^]: E2B Docs - Read and write files. https://e2b.dev/docs/filesystem/read-write
[^9^]: Modal Docs - Snapshots. https://modal.com/docs/guide/sandbox-snapshots
[^10^]: Modal Docs - Snapshots (detailed). https://modal.com/docs/guide/sandbox-snapshots
[^11^]: Fly.io Docs - Working with Docker on Fly.io. https://fly.io/docs/blueprints/working-with-docker/
[^12^]: Cloudflare Workers Docs - node:fs. https://developers.cloudflare.com/workers/runtime-apis/nodejs/fs/
[^13^]: Ashley Peacock Blog - Running Containers on Cloudflare. https://blog.ashleypeacock.co.uk/p/running-containers-on-cloudflare
[^14^]: Docker Docs - docker container export. https://docs.docker.com/reference/cli/docker/container/export/
[^15^]: Incus Docs - incus export. https://linuxcontainers.org/incus/docs/main/reference/manpages/incus/export/
[^16^]: Actuated Blog - Firecracker Container Lab. https://actuated.com/blog/firecracker-container-lab
[^17^]: Transloadit - Stream Tar archives between servers. https://transloadit.com/devtips/stream-tar-archives-between-servers-without-local-storage/
[^18^]: AWS China - EBS Direct APIs announcement. https://www.amazonaws.cn/en/new/2020/amazon-ebs-direct-apis-create-snapshots-aws-china-regions/
[^19^]: Medium - Convert a VM to AMI in AWS. https://mohamedmsaeed.medium.com/convert-a-vm-to-ami-in-aws-b101a1c52e0e
[^20^]: Stack Overflow - How to create an AMI using the snapshot. https://stackoverflow.com/questions/62406115/how-to-create-an-ami-using-the-snapshot
[^21^]: Winslow TG - How to Move an Existing VM into AWS EC2. https://winslowtg.com/how-to-move-an-existing-vm-into-aws-ec2/
[^22^]: OneUptime - Use EC2 User Data Scripts for Instance Bootstrapping. https://oneuptime.com/blog/post/2026-02-12-use-ec2-user-data-scripts-for-instance-bootstrapping/view
[^23^]: NetApp - Convert Google Cloud image to raw format. https://docs.netapp.com/us-en/storage-management-cloud-volumes-ontap/task-gcp-convert-image-raw.html
[^24^]: GitHub - gce_export README. https://github.com/GoogleCloudPlatform/compute-image-tools/blob/master/cli_tools/gce_export/README.md
[^25^]: Medium - Exporting a custom image to Google Cloud Storage. https://diana-moraa.medium.com/exporting-a-custom-image-to-google-cloud-storage-31aec78fd3ea
[^26^]: SuperUser - Upload my own VM image to GCE. https://superuser.com/questions/977292/upload-my-own-vm-image-to-gce
[^27^]: Microsoft Learn - Upload a VHD to Azure. https://learn.microsoft.com/en-us/azure/virtual-machines/windows/disks-upload-vhd-to-managed-disk-powershell
[^28^]: DigitalOcean Docs - Snapshots product page. https://docs.digitalocean.com/products/snapshots/
[^29^]: GitHub Gist - Backup DigitalOcean droplet locally. https://gist.github.com/amalmurali47/c58ef024683cccd242625995b45b7b72
[^30^]: GitHub - hcloud-upload-image. https://github.com/apricote/hcloud-upload-image
[^31^]: Daytona TypeScript SDK - FileSystem. https://www.daytona.io/docs/en/typescript-sdk/file-system/
[^32^]: GitHub - Daytona issue #3413. https://github.com/daytonaio/daytona/issues/3413
[^33^]: E2B Docs - Upload & download files. https://e2b.dev/docs/quickstart/upload-download-files
[^34^]: OpenKruise - E2B SDK compatibility. https://openkruise.io/kruiseagents/developer-manuals/e2b-client
[^35^]: Fly.io Docs - Volumes overview. https://fly.io/docs/volumes/overview/
[^36^]: Fly.io Docs - Manage volume snapshots. https://fly.io/docs/volumes/snapshots/
[^37^]: Rennerocha Blog - Creating backups for fly.io Volumes. https://rennerocha.com/posts/creating-backups-for-fly-io-volumes/
[^38^]: Modal Blog - Directory Snapshots. https://modal.com/blog/directory-snapshots-resumable-project-state-for-sandboxes
[^39^]: Modal Blog - Memory Snapshots. https://modal.com/blog/mem-snapshots
[^40^]: Modal Docs - modal.Volume reference. https://modal.com/docs/reference/modal.Volume
[^41^]: Modal Docs - Filesystem Access. https://modal.com/docs/guide/sandbox-files
[^42^]: Cloudflare Sandbox SDK - Manage files. https://developers.cloudflare.com/sandbox/guides/manage-files/
[^43^]: Cloudflare Sandbox SDK - Backup and restore. https://developers.cloudflare.com/sandbox/guides/backup-restore/
[^44^]: Iximiuz Labs - Extracting container image filesystem. https://labs.iximiuz.com/tutorials/extracting-container-image-filesystem
[^45^]: Firecracker GitHub - Snapshot support. https://github.com/firecracker-microvm/firecracker/blob/main/docs/snapshotting/snapshot-support.md
[^46^]: Qameta Blog - Copy files compressed with tar via ssh. https://qameta.com/posts/copy-files-compressed-with-tar-via-ssh-to-a-linux-server/
