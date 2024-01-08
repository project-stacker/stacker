---
marp: true
theme: gaia
---

# Open Standards For Datacenter Software

Ramkumar Chinchani
rchincha@cisco.com

---

# Why Open Standards?

* Avoid vendor lock-in
* Large ecosystem
* Pace of innovation

---

![width:100%](standards-bodies.png)

---

# Regulatory Requirements

* [Executive Order on Improving the Nation’s Cybersecurity](https://www.whitehouse.gov/briefing-room/presidential-actions/2021/05/12/executive-order-on-improving-the-nations-cybersecurity/)
    * software bill of materials (SBOM) - SPDX

* [FedRamp](https://www.fedramp.gov)
    * [FEDRAMP VULNERABILITY SCANNING REQUIREMENTS FOR CONTAINERS](https://www.fedramp.gov/assets/resources/documents/Vulnerability_Scanning_Requirements_for_Containers.pdf)

* [NIST](https://www.nist.gov/)
    * [NIST Special Publication 800-190/Application Container Security Guide](https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-190.pdf)

---

# `CNCF` Ecosystem
https://cncf.landscape2.io/

![width:800px](cncf-landscape2.png)

---

# `OCI` "Standards"

* _image spec_
    * https://github.com/opencontainers/image-spec
* _runtime spec_
    * https://github.com/opencontainers/runtime-spec
* _distribution spec_
    * https://github.com/opencontainers/distribution-spec

---

# `OCI` Ecosystem

| Purpose | Redhat | Microsoft | Google | Docker| Cisco |
| --- | --- | --- | --- | --- | --- |
| Build | `buildah` | | `bazel` | `buildx` | `stacker` |
| Push/pull | `skopeo` | `oras` | `crane` | _`docker`_ | |
| Run | `podman` | | | `docker` | | 
| Sign | `cosign` | `notation` | `cosign` | `notaryv1` | |
| Registry | _`quay`_ | `acr` | _`gar`_ | _`distribution`_ | `zot` |

---

# `CNCF` Meets `OCI`

![](cri-ecosystem.png)

---

# Putting Everything Together

![](flow.png)