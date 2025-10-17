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



