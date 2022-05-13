# Architecture of Dagu

Dagu is self-contained and simple architecture. It uses Unix sockets to communicate with running processes. You may wonder how it store the data? Dagu stores execution history data as plain simple JSON files in the local directory. Each JSON files contain all necessary information for later retry of each workflow run.

![dagu Architecture](https://user-images.githubusercontent.com/1475839/166390371-00bb4af0-3689-406a-a4d5-af943a1fd2ce.png)
