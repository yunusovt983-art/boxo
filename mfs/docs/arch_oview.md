# Обзор пакета `mfs`

## Назначение
- Поддерживает in-memory модель Mutable File System поверх UnixFS/IPLD.
- Предоставляет деревообразное API, синхронизируя изменения через `ipld.DAGService`.
- По желанию объявляет новые CID в маршрутизации Libp2p через `routing.ContentProviding`.

## Ключевые структуры
- **Root** (`root.go`): хранит корневую директорию, управляет (пере)публикацией и инициализацией дерева из существующего или пустого узла.
- **Directory** (`dir.go`): оборачивает UnixFS-директорию, кэширует дочерние элементы, синхронизирует их с DAG и поднимает изменения вверх.
- **File** (`file.go`): представляет UnixFS-файл, координирует конкурентный доступ через дескрипторы и обновляет родителя после `Flush`.
- **inode** (`inode.go`): общая база для файлов и директорий (имя, родитель, `DAGService`, провайдер).
- **FSNode / NodeType** (`root.go`): унифицированный интерфейс для работы с файлами и директориями как с абстрактными узлами дерева.

## Работа с файлами
- `File.Open` возвращает `FileDescriptor`, обёрнутый над `unixfs/mod.DagModifier` для чтения/записи/обрезки.
- `FileDescriptor` (`fd.go`) управляет состояниями (`Created`, `Dirty`, `Flushed`, `Closed`), следит за блокировками и отвечает за `Flush`.
- `Flags` (`options.go`) задаёт режимы открытия (Read/Write/Sync).
- Методы `File` для `SetMode`, `SetModTime`, `Flush` обновляют UnixFS-узел и оповещают родителя.

## Высокоуровневые операции
- `ops.go` реализует POSIX-подобные функции: `Mv`, `PutNode`, `Mkdir` (с `Mkparents`), `Lookup`, `FlushPath`, `Chmod`, `Touch`.
- Все операции начинают с `Root`, находят нужные `Directory`/`File` через `Lookup` и синхронизируют изменения вверх.

## Републикация и предоставление
- `Republisher` (`repub.go`) агрегирует изменения CID, вызывает пользовательский `PubFunc`, управляет таймерами (быстрый/длинный) и ретраит ошибки.
- `Root` и `Directory` после записи в `DAGService` вызывают `Provide`, если передан `routing.ContentProviding`.

## Тесты
- `mfs_test.go` покрывает жизненный цикл файлов/директорий, конкуренцию и согласованность кэша.
- `repub_test.go` проверяет работу `Republisher`, таймауты и ожидание публикации.

## Полезные замечания
- `Directory.cacheSync` синхронизирует кэш и может его очищать при полном `Flush`.
- `File.setNodeData` переупаковывает данные в UnixFS-протоузел и поднимает изменения к родителю.
- TODO-комментарии подчёркивают потенциальные точки рефакторинга (слияние `Root`/`Directory`, документирование `fullsync`, упрощение работы с `chunker`).
  
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
<img width="702" height="606" alt="FD-StateMachine" src="https://github.com/user-attachments/assets/715d57a2-f155-4f77-b996-294f439bd434" />

```go
@startuml FD-StateMachine
' State machine for FileDescriptor lifecycle
skinparam defaultFontColor #1D2939
skinparam roundcorner 12


[*] --> Created
Created --> Dirty : Write/Truncate
Dirty --> Dirty : More writes
Dirty --> Flushed : Flush -> DAG.Put OK
Flushed --> Dirty : Write/Truncate again
Created --> Closed : Close (no writes)
Flushed --> Closed : Close
Dirty --> Closed : Close (implicit Flush?)
Closed --> [*]


note right of Dirty
Holds mutex during mutations
DagModifier accumulates changes
end note


note bottom of Flushed
Parent notified to update links
Republisher may be triggered
end note
@enduml
```


<img width="1449" height="1504" alt="C4-Context-MFS" src="https://github.com/user-attachments/assets/730fb302-ead6-49ab-a0ca-4cc9a1b5eea8" />

```go
@startuml C4-Context-MFS
!include <C4/C4_Context>
LAYOUT_TOP_DOWN()
skinparam defaultFontColor #1D2939
skinparam ArrowColor #667085
skinparam roundcorner 14


AddElementTag("ext", $bgColor="#F2F4F7", $borderColor="#D0D5DD", $fontColor="#101828")
AddElementTag("core", $bgColor="#E6F4EA", $borderColor="#16A34A", $fontColor="#07270F")


Person(user, "Client App / Tooling", "CLI/Daemon/Service использует boxo/mfs")
System_Boundary(boxo, "boxo (Go libs)") {
System(mfs, "mfs library", "Mutable File System", $tags="core")
}
System_Ext(ipld, "IPLD DAGService", "Чтение/запись узлов", $tags="ext")
System_Ext(unixfs, "UnixFS", "ФС-представление поверх IPLD", $tags="ext")
System_Ext(routing, "Libp2p Routing", "ContentProviding / announce CIDs", $tags="ext")
System_Ext(blockstore, "Blockstore / Datastore", "Хранение блоков", $tags="ext")


Rel(user, mfs, "POSIX-подобные операции (Lookup/Mkdir/Put/Flush)")
Rel(mfs, ipld, "Put/Get nodes")
Rel(mfs, unixfs, "Инкапсулирует файлы/директории")
Rel(mfs, routing, "Provide root CID (optional)")
Rel(ipld, blockstore, "persist blocks")
@enduml


'-----------------------------
@startuml C4-Container-MFS
!include <C4/C4_Container>
LAYOUT_TOP_DOWN()
skinparam defaultFontColor #1D2939
skinparam ArrowColor #667085
skinparam roundcorner 14


AddElementTag("lib", $bgColor="#E6F4EA", $borderColor="#22C55E", $fontColor="#052E16")
AddElementTag("ext", $bgColor="#F2F4F7", $borderColor="#D0D5DD", $fontColor="#101828")


System_Boundary(app, "Application using boxo/mfs") {
Container(appsvc, "App Service/Daemon", "Go", "Вызывает API mfs (ops.go)")
}


System_Boundary(mfsb, "boxo/mfs") {
Container(rootc, "Root", "Go", "Инициализация дерева, корневой каталог", $tags="lib")
Container(dirc, "Directory", "Go", "Кэш узлов, синхронизация вверх", $tags="lib")
Container(filec, "File", "Go", "UnixFS-файл, Flush/Truncate", $tags="lib")
Container(fdc, "FileDescriptor", "Go", "Конкурентный доступ через DagModifier", $tags="lib")
Container(opsc, "ops API", "Go", "Mv/Mkdir/PutNode/Chmod/Touch/FlushPath", $tags="lib")
Container(repubc, "Republisher", "Go", "Агрегация CID, короткий/длинный таймер", $tags="lib")
}


Container_Ext(dag, "ipld.DAGService", "Go", "Put/Get nodes", $tags="ext")
Container_Ext(unix, "unixfs", "Go", "FSNode/DagModifier", $tags="ext")
Container_Ext(provider, "routing.ContentProviding", "Go", "Provide CIDs", $tags="ext")
Container_Ext(store, "Blockstore/Datastore", "Go", "Persist blocks", $tags="ext")


Rel(appsvc, opsc, "Вызовы API")
Rel(opsc, rootc, "resolve & dispatch")
Rel(rootc, dirc, "GetDirectory()/Lookup")
Rel(dirc, filec, "leaf operations")
Rel(filec, fdc, "Open() → FD")
Rel(fdc, unix, "DagModifier RW")
Rel(filec, dag, "Flush → Put(node)")
Rel(dirc, dag, "Put(dir)")
Rel(dag, store, "blocks")
Rel(rootc, repubc, "Trigger(cid)")
Rel(repubc, provider, "PubFunc(cid)")
@enduml
```
<img width="781" height="1437" alt="C4-Container-MFS" src="https://github.com/user-attachments/assets/382c6453-7786-45ab-94b2-df17d97cc679" />

```go
@startuml C4-Container-MFS
!include <C4/C4_Container>
LAYOUT_TOP_DOWN()
skinparam defaultFontColor #1D2939
skinparam ArrowColor #667085
skinparam roundcorner 14


AddElementTag("lib", $bgColor="#E6F4EA", $borderColor="#22C55E", $fontColor="#052E16")
AddElementTag("ext", $bgColor="#F2F4F7", $borderColor="#D0D5DD", $fontColor="#101828")


System_Boundary(app, "Application using boxo/mfs") {
Container(appsvc, "App Service/Daemon", "Go", "Вызывает API mfs (ops.go)")
}


System_Boundary(mfsb, "boxo/mfs") {
Container(rootc, "Root", "Go", "Инициализация дерева, корневой каталог", $tags="lib")
Container(dirc, "Directory", "Go", "Кэш узлов, синхронизация вверх", $tags="lib")
Container(filec, "File", "Go", "UnixFS-файл, Flush/Truncate", $tags="lib")
Container(fdc, "FileDescriptor", "Go", "Конкурентный доступ через DagModifier", $tags="lib")
Container(opsc, "ops API", "Go", "Mv/Mkdir/PutNode/Chmod/Touch/FlushPath", $tags="lib")
Container(repubc, "Republisher", "Go", "Агрегация CID, короткий/длинный таймер", $tags="lib")
}


Container_Ext(dag, "ipld.DAGService", "Go", "Put/Get nodes", $tags="ext")
Container_Ext(unix, "unixfs", "Go", "FSNode/DagModifier", $tags="ext")
Container_Ext(provider, "routing.ContentProviding", "Go", "Provide CIDs", $tags="ext")
Container_Ext(store, "Blockstore/Datastore", "Go", "Persist blocks", $tags="ext")


Rel(appsvc, opsc, "Вызовы API")
Rel(opsc, rootc, "resolve & dispatch")
Rel(rootc, dirc, "GetDirectory()/Lookup")
Rel(dirc, filec, "leaf operations")
Rel(filec, fdc, "Open() → FD")
Rel(fdc, unix, "DagModifier RW")
Rel(filec, dag, "Flush → Put(node)")
Rel(dirc, dag, "Put(dir)")
Rel(dag, store, "blocks")
Rel(rootc, repubc, "Trigger(cid)")
Rel(repubc, provider, "PubFunc(cid)")
@enduml
```



