# DC

**DC** (Decentralized Cloud) is a versatile decentralized cloud service known as Depin. It empowers traditional developers to create Web3.0 applications with the same ease as developing conventional internet applications. Simultaneously, it removes the hurdles for traditional internet users, allowing them to navigate Web3.0 applications as effortlessly as they would with conventional ones. This approach enables users to smoothly transition into the Web3.0 realm before diving into its specialized knowledge.  
The **DC**  network is structured around both core and auxiliary components. The core component is made up of the cloud service node, DCStorage, and the blockchain, DCChain. On the other hand, the auxiliary component encompasses a variety of tools and programs: DCUpgrade, which assists in updating DCStorage; DCManager, a management tool for DC nodes; DappSDK, which is offered to DApp developers; and DC_Debugenv, a local debugging environment for DApps, among others.  

- **DCChain**: DCChain is a public blockchain developed  based on Substrate, with its core functions concentrated in a pallet called DCNode, which serves as the incentive layer and consensus layer of the DC network. It is responsible for maintaining the consensus data across the entire network, including subscription information for user cloud service spaces, binding relationships between user accounts, associations between users and files/databases/NftAccounts, storage locations of files and databases, basic information of cloud service nodes, and reward information for various participating parties.
  
- **DCStorage**: Nodes running DCStorage within the DC network are called cloud service nodes. They operate on Intel SGX Enclaves as Trusted Execution Environments (TEEs), providing web3.0 applications with capabilities similar to traditional cloud services, including file storage, database management and access, inter-user communication routing, user message data caching, and submission of consensus-requiring data to the blockchain. Web3.0 applications built on DC can directly invoke the decentralized capabilities offered by DCStorage via the gRPC protocol, thus achieving high performance and concurrency at scale - application experiences not possible in the original blockchain smart contract model.
- **DAppSDK**:A complete set of gRPC interfaces and client SDKs are provided, allowing developers to create web3.0 applications as if they were developing web2.0 applications, without the need to understand blockchain technology. Developers can simply refer to the interfaces provided by DC for rapid development of web3.0 applications.
- **DC_DebugEnv**: Deployed based on Docker, the entire DC test network can be set up locally for development and debugging. Once the DApp development is complete, it only requires switching the blockchain address to the official DC network to go live and launch.
- **DCManager**: The DC network node deployment and maintenance tools assist users in quickly setting up and operating DC network nodes, and provide monitoring and maintenance functions for the nodes.
- **DCUpgrade**: The DC network node upgrade tool is mainly responsible for assisting in the transition and transfer of keys sealed by TEE during the DCStorage upgrade process. The program itself also runs in a TEE.

## Preparation work

- Hardware requirements:
  
  CPU must support SGX 2.0 and EPC size must be greater than or equal to 64G, also ensure that SGX feature is enabled in the BIOS.  
  Intel's third-generation (Ice Lake) Xeon Scalable processors, as well as the majority of the fourth-generation (Sapphire Rapids) Xeon Scalable processors,including the Silver, Gold, and Platinum tiers support SGX 2.0 and EPC size is greater than or equal to 64G.

- Operating system requirements:

  Ubuntu 22.04
  
- Other configurations

  - **Secure Boot** in BIOS needs to be turned off
  - If need to run DCStorage,you should register with [Intel](https://api.portal.trustedservices.intel.com/provisioning-certification)(Click on 'Subscribe' in the page) to get a PCCS API key.This key will be used to config to **DC** service.

## Install dependencies

### Install **DC** service

```shell
sudo ./install.sh # Use 'sudo ./install.sh --registry cn' to accelerate installation in china 
. /etc/bash_completion # to enable command completion
```

### Config service

- Config service
  
    ```shell
    dc config
    ```

  First select the boot mode according to the prompt, there are mainly two modes: validator mode and normal node mode; Next, configure the PCCS API key subscribed from the [Intel website](https://api.portal.trustedservices.intel.com/provisioning-certification)  according to the prompts (if not configured separately).

- Check PCCS API key
  
  ```shell
  dc pccs_api_key
  ```

- Config PCCS API key separately

  ```shell
  dc pccs_api_key <pccs_api_key>
  ```

  pccs_api_key is the key you get from Intel

### Run service

- Please make sure the following ports are not occupied before starting：
  - 60666 9944   (for DCChain)
  - 6667 4006 4016 4026(for DCStorage)
  - 6666  (for DCUpgrade)
  - 8081  (for PCCS)

- View command help information
  
  ```shell
  dc help
  ```

- Start Service

  ```shell
   dc start  {storage|chain|all} 
  ```

- Check service status

  ```shell
  dc status
  ```

- View service log

  ```shell
  dc log  {storage|chain|upgrade|pccs} [num] 
  ```

- View service version information, and the current node's EnclaveID for DCStorage

  ```shell
  dc uniqueid
  ```

- View the node information of the current network

  ```shell
  dc peerinfo
  ```

- Stop service

  ```shell
  dc stop {storage|chain|all}
  ```

### Uninstall service
  
  ```shell
  cd /opt/dcnetio/bin
  sudo ./uninstall.sh
  ```

## License

[MIT](LICENSE)
