# Planned use cases

### 1. Security update
A security vulnerability is identified in an installed native workload such as "containerd", and a patched version is available. The fleet manager shows which devices are still using an affected version. The update is started manually for all selected devices. Afterwards, the fleet manager displays the installation status and the resulting version.

### 2. New native workload version required
A newer approved version of a system library such as "OpenSSL" is required. The fleet manager shows which version is installed on each device and which devices do not yet meet the desired version level. The update is started manually for all selected devices. Afterwards, the fleet manager displays the installation status and the resulting version.

### 3. Installation of a new native workload
An additional native workload such as "nmap" is to be made available on the devices. The fleet manager shows which devices do not yet have the native workload installed. The installation is started manually for all selected devices. Afterwards, the fleet manager displays the installation status and the resulting version.

### 4. Update failure
A native workload cannot be installed, for example due to an unmet dependency, insufficient storage space or an unreachable source. The desired state is native workload "B", but native workload "A" remains installed on the device and the update receives the status "FAILED". Afterwards, the fleet manager displays the installation status and the resulting version.

### 5. Native workload rollback
A native workload has been updated from version "A" to version "B". It is then found that version "B" does not work as expected. The fleet manager shows which devices are already using version "B". The rollback is started manually for all selected devices. Afterwards, the fleet manager displays the installation status and the resulting version.

### 6. Native workload uninstallation
A previously required native workload is no longer used, for example because an application has been removed or a function has been replaced. The fleet manager shows which devices still have the native workload installed. The uninstallation is started manually for all selected devices. Afterwards, the fleet manager displays the installation status.

### 7. Updating related native workloads together
A component consists of several interdependent native workloads. For the Docker stack, these include, for example, docker-ce, docker-ce-cli, containerd.io, docker-compose-plugin and docker-buildx-plugin. In the fleet manager, these native workloads are managed as a related native workload group or as a single deployment. The update is started manually for all selected devices. Afterwards, the fleet manager displays the installation status and the resulting versions.