
<img width="1252" height="876" alt="MFS-Write-Flush-Sequence" src="https://github.com/user-attachments/assets/216b8c83-d3eb-4bcc-85ab-dd8330db1015" />

```go
@startuml MFS-Write-Flush-Sequence
' Sequence: open → write → flush cascades → provide
skinparam defaultFontColor #1D2939
skinparam sequenceArrowThickness 1
skinparam roundcorner 12


actor Client
participant Root
participant Directory as Dir
participant File
participant FileDescriptor as FD
participant "DagModifier" as DM
participant "DAGService" as DAG
participant Republisher as Rep
participant Provider as Prov


Client -> Root : Lookup("/path/to/file")
Root -> Dir : Lookup("/path/to")
Dir --> Client : *File
Client -> File : Open(Flags{Write,Sync})
File -> FD : create FD
FD -> DM : init on current UnixFS node
Client -> FD : Write(bytes)
FD -> DM : WriteAt/Append
FD -> FD : state=Dirty
...
Client -> FD : Flush()
FD -> DM : Sync()
FD -> File : setNodeData(updated FSNode)
File -> DAG : Put(updated node)
File -> Dir : touchParent()
Dir -> DAG : Put(updated dir)
Dir -> Root : bubble CID up
Root -> Rep : Trigger(newRootCID)
Rep -> Prov : PubFunc(CID)
Prov --> Rep : ok
Rep --> Root : ack
Client -> FD : Close()
FD -> FD : state=Closed
@enduml
```

