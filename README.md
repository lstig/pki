# PKI

Instructions and setup for air-gapped PKI.

## Pre-requisites

- [Task](https://taskfile.dev)
- Docker
- 16 GB SD Card/USB Drive (minimum)
- Raspberry Pi 3B+ (or later)

## Setup

### Determine the correct disk

Connect the SD Card or USB drive to your computer and determine the appropriate `/dev/$ID`.
On MacOS the device can be located with `diskutil`.

```shell
diskutil list physical
```

You should see a disk that matches the size of the device you inserted (in the following example `/dev/disk6`):

```text
/dev/disk0 (internal, physical):
   #:                       TYPE NAME                    SIZE       IDENTIFIER
   0:      GUID_partition_scheme                        *500.3 GB   disk0
   ...

/dev/disk6 (internal, physical):
   #:                       TYPE NAME                    SIZE       IDENTIFIER
   0:     FDisk_partition_scheme                        *31.7 GB    disk6
   ...
```

### Write the image to the disk

> [!CAUTION]
> **CAREFULLY** review the output of `diskutil list`. 
> The following command could cause *irreparable* harm if it is run against the wrong disk.

Build the image and write it to the disk.

```shell
# <identifier> should be replaced with your disk's identifier.
# Using the example above: <identifier>=disk6
task write:<identifier>
```
