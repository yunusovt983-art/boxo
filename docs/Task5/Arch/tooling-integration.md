# Интеграция архитектурных диаграмм с инструментами разработки

## 🛠️ IDE Integration

### VS Code Extension для работы с диаграммами

#### Создание расширения для VS Code
```json
// .vscode/extensions/boxo-architecture/package.json
{
  "name": "boxo-architecture",
  "displayName": "Boxo Architecture Helper",
  "description": "Помощник для работы с архитектурными диаграммами Boxo",
  "version": "1.0.0",
  "engines": {
    "vscode": "^1.60.0"
  },
  "categories": ["Other"],
  "activationEvents": [
    "onLanguage:go",
    "onCommand:boxo.showArchitecture"
  ],
  "main": "./out/extension.js",
  "contributes": {
    "commands": [
      {
        "command": "boxo.showArchitecture",
        "title": "Show Architecture Diagram",
        "category": "Boxo"
      },
      {
        "command": "boxo.validateArchitecture",
        "title": "Validate Code Against Architecture",
        "category": "Boxo"
      },
      {
        "command": "boxo.generateInterface",
        "title": "Generate Interface from Diagram",
        "category": "Boxo"
      }
    ],
    "menus": {
      "explorer/context": [
        {
          "command": "boxo.showArchitecture",
          "when": "resourceExtname == .puml",
          "group": "boxo"
        }
      ],
      "editor/context": [
        {
          "command": "boxo.validateArchitecture",
          "when": "resourceLangId == go",
          "group": "boxo"
        }
      ]
    },
    "configuration": {
      "title": "Boxo Architecture",
      "properties": {
        "boxo.diagramsPath": {
          "type": "string",
          "default": "docs/Task5/Arch",
          "description": "Path to architecture diagrams"
        },
        "boxo.autoValidate": {
          "type": "boolean",
          "default": true,
          "description": "Automatically validate code against architecture"
        }
      }
    }
  }
}
```

#### Реализация расширения
```typescript
// .vscode/extensions/boxo-architecture/src/extension.ts
import * as vscode from 'vscode';
import * as path from 'path';
import * as fs from 'fs';

export function activate(context: vscode.ExtensionContext) {
    // Команда для показа архитектурной диаграммы
    const showArchitectureCommand = vscode.commands.registerCommand('boxo.showArchitecture', async (uri: vscode.Uri) => {
        const panel = vscode.window.createWebviewPanel(
            'boxoArchitecture',
            'Boxo Architecture',
            vscode.ViewColumn.Beside,
            {
                enableScripts: true,
                localResourceRoots: [vscode.Uri.file(path.join(context.extensionPath, 'media'))]
            }
        );

        const diagramContent = fs.readFileSync(uri.fsPath, 'utf8');
        const htmlContent = generateDiagramHTML(diagramContent, context);
        panel.webview.html = htmlContent;
    });

    // Команда для валидации архитектуры
    const validateArchitectureCommand = vscode.commands.registerCommand('boxo.validateArchitecture', async () => {
        const activeEditor = vscode.window.activeTextEditor;
        if (!activeEditor) {
            return;
        }

        const document = activeEditor.document;
        const violations = await validateCodeAgainstArchitecture(document);
        
        if (violations.length > 0) {
            showArchitectureViolations(violations, activeEditor);
        } else {
            vscode.window.showInformationMessage('Code complies with architecture!');
        }
    });

    // Автоматическая валидация при сохранении
    const onSaveValidation = vscode.workspace.onDidSaveTextDocument(async (document) => {
        const config = vscode.workspace.getConfiguration('boxo');
        if (config.get('autoValidate') && document.languageId === 'go') {
            const violations = await validateCodeAgainstArchitecture(document);
            if (violations.length > 0) {
                vscode.window.showWarningMessage(`Found ${violations.length} architecture violations`);
            }
        }
    });

    context.subscriptions.push(showArchitectureCommand, validateArchitectureCommand, onSaveValidation);
}

function generateDiagramHTML(diagramContent: string, context: vscode.ExtensionContext): string {
    return `
    <!DOCTYPE html>
    <html>
    <head>
        <meta charset="UTF-8">
        <title>Architecture Diagram</title>
        <script src="https://unpkg.com/plantuml-encoder@1.4.0/dist/plantuml-encoder.min.js"></script>
    </head>
    <body>
        <div id="diagram-container">
            <img id="diagram" src="" alt="Loading diagram..." />
        </div>
        <script>
            const diagramSource = \`${diagramContent}\`;
            const encoded = plantumlEncoder.encode(diagramSource);
            const diagramUrl = 'https://www.plantuml.com/plantuml/svg/' + encoded;
            document.getElementById('diagram').src = diagramUrl;
        </script>
    </body>
    </html>
    `;
}

interface ArchitectureViolation {
    line: number;
    column: number;
    message: string;
    severity: 'error' | 'warning' | 'info';
    rule: string;
}

async function validateCodeAgainstArchitecture(document: vscode.TextDocument): Promise<ArchitectureViolation[]> {
    const violations: ArchitectureViolation[] = [];
    const text = document.getText();
    const lines = text.split('\n');

    // Проверка соответствия интерфейсам из Code Diagram
    const interfacePattern = /type\s+(\w+)\s+interface\s*{/g;
    let match;
    
    while ((match = interfacePattern.exec(text)) !== null) {
        const interfaceName = match[1];
        const lineNumber = text.substring(0, match.index).split('\n').length - 1;
        
        if (!isInterfaceInDiagram(interfaceName)) {
            violations.push({
                line: lineNumber,
                column: match.index - text.lastIndexOf('\n', match.index) - 1,
                message: `Interface '${interfaceName}' not found in architecture diagrams`,
                severity: 'warning',
                rule: 'interface-compliance'
            });
        }
    }

    // Проверка зависимостей согласно Component Diagram
    const importPattern = /import\s+(?:"([^"]+)"|`([^`]+)`)/g;
    while ((match = importPattern.exec(text)) !== null) {
        const importPath = match[1] || match[2];
        const lineNumber = text.substring(0, match.index).split('\n').length - 1;
        
        if (!isDependencyAllowed(document.fileName, importPath)) {
            violations.push({
                line: lineNumber,
                column: match.index - text.lastIndexOf('\n', match.index) - 1,
                message: `Dependency '${importPath}' violates component architecture`,
                severity: 'error',
                rule: 'dependency-compliance'
            });
        }
    }

    return violations;
}

function showArchitectureViolations(violations: ArchitectureViolation[], editor: vscode.TextEditor) {
    const diagnostics: vscode.Diagnostic[] = violations.map(violation => {
        const range = new vscode.Range(
            new vscode.Position(violation.line, violation.column),
            new vscode.Position(violation.line, violation.column + 10)
        );
        
        const diagnostic = new vscode.Diagnostic(
            range,
            violation.message,
            violation.severity === 'error' ? vscode.DiagnosticSeverity.Error :
            violation.severity === 'warning' ? vscode.DiagnosticSeverity.Warning :
            vscode.DiagnosticSeverity.Information
        );
        
        diagnostic.source = 'boxo-architecture';
        diagnostic.code = violation.rule;
        
        return diagnostic;
    });

    const collection = vscode.languages.createDiagnosticCollection('boxo-architecture');
    collection.set(editor.document.uri, diagnostics);
}
```

### IntelliJ IDEA Plugin

#### Конфигурация плагина
```xml
<!-- .idea/plugins/boxo-architecture/META-INF/plugin.xml -->
<idea-plugin>
    <id>com.boxo.architecture</id>
    <name>Boxo Architecture Helper</name>
    <version>1.0.0</version>
    <vendor>Boxo Team</vendor>
    
    <description><![CDATA[
        Помощник для работы с архитектурными диаграммами Boxo.
        Предоставляет валидацию кода, навигацию по архитектуре и генерацию кода.
    ]]></description>
    
    <depends>com.intellij.modules.platform</depends>
    <depends>org.jetbrains.plugins.go</depends>
    
    <extensions defaultExtensionNs="com.intellij">
        <toolWindow id="BoxoArchitecture" 
                   secondary="true" 
                   anchor="right" 
                   factoryClass="com.boxo.architecture.ArchitectureToolWindowFactory"/>
        
        <inspectionToolProvider implementation="com.boxo.architecture.ArchitectureInspectionProvider"/>
        
        <intentionAction>
            <className>com.boxo.architecture.GenerateInterfaceIntention</className>
            <category>Boxo Architecture</category>
        </intentionAction>
    </extensions>
    
    <actions>
        <group id="BoxoArchitectureActions" text="Boxo Architecture" popup="true">
            <add-to-group group-id="ToolsMenu" anchor="last"/>
            
            <action id="ValidateArchitecture" 
                   class="com.boxo.architecture.ValidateArchitectureAction" 
                   text="Validate Against Architecture"/>
            
            <action id="ShowArchitectureDiagram" 
                   class="com.boxo.architecture.ShowDiagramAction" 
                   text="Show Architecture Diagram"/>
        </group>
    </actions>
</idea-plugin>
```

## 🔄 CI/CD Integration

### GitHub Actions для валидации архитектуры

```yaml
# .github/workflows/architecture-check.yml
name: Architecture Compliance Check

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  architecture-validation:
    runs-on: ubuntu-latest
    
    steps:
    - name: Checkout code
      uses: actions/checkout@v3
      
    - name: Setup Go
      uses: actions/setup-go@v3
      with:
        go-version: '1.21'
        
    - name: Install PlantUML
      run: |
        sudo apt-get update
        sudo apt-get install -y plantuml
        
    - name: Validate diagram syntax
      run: |
        cd docs/Task5/Arch
        for file in *.puml; do
          echo "Validating $file..."
          plantuml -checkonly "$file"
        done
        
    - name: Build architecture validator
      run: |
        cd tools/arch-validator
        go build -o arch-validator .
        
    - name: Run architecture validation
      run: |
        ./tools/arch-validator/arch-validator \
          --diagrams-path docs/Task5/Arch \
          --code-path bitswap/monitoring \
          --output-format github-actions \
          --fail-on-violations
          
    - name: Generate architecture report
      if: always()
      run: |
        ./tools/arch-validator/arch-validator \
          --diagrams-path docs/Task5/Arch \
          --code-path bitswap/monitoring \
          --output-format html \
          --output-file architecture-report.html
          
    - name: Upload architecture report
      if: always()
      uses: actions/upload-artifact@v3
      with:
        name: architecture-report
        path: architecture-report.html
        
    - name: Comment PR with violations
      if: github.event_name == 'pull_request' && failure()
      uses: actions/github-script@v6
      with:
        script: |
          const fs = require('fs');
          const violations = JSON.parse(fs.readFileSync('violations.json', 'utf8'));
          
          let comment = '## 🏗️ Architecture Compliance Issues\n\n';
          comment += `Found ${violations.length} architecture violations:\n\n`;
          
          violations.forEach(violation => {
            comment += `- **${violation.type}**: ${violation.description}\n`;
            comment += `  - File: \`${violation.file}\`\n`;
            comment += `  - Line: ${violation.line}\n\n`;
          });
          
          comment += '\nPlease review the architecture diagrams and update your code accordingly.';
          
          github.rest.issues.createComment({
            issue_number: context.issue.number,
            owner: context.repo.owner,
            repo: context.repo.repo,
            body: comment
          });
```

### GitLab CI для архитектурной валидации

```yaml
# .gitlab-ci.yml
stages:
  - validate
  - build
  - test
  - deploy

architecture-validation:
  stage: validate
  image: golang:1.21-alpine
  
  before_script:
    - apk add --no-cache openjdk11-jre-headless wget
    - wget -O plantuml.jar https://github.com/plantuml/plantuml/releases/latest/download/plantuml-1.2024.0.jar
    
  script:
    # Валидация синтаксиса диаграмм
    - cd docs/Task5/Arch
    - for file in *.puml; do java -jar ../../../plantuml.jar -checkonly "$file"; done
    
    # Сборка и запуск валидатора архитектуры
    - cd ../../../tools/arch-validator
    - go build -o arch-validator .
    - ./arch-validator --diagrams-path ../../docs/Task5/Arch --code-path ../../bitswap/monitoring --output-format junit --output-file ../../architecture-results.xml
    
  artifacts:
    reports:
      junit: architecture-results.xml
    paths:
      - architecture-results.xml
    expire_in: 1 week
    
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
    - if: $CI_COMMIT_BRANCH == "main"
    - if: $CI_COMMIT_BRANCH == "develop"
```

## 📊 Monitoring и Metrics

### Prometheus метрики для архитектурного соответствия

```go
// tools/arch-metrics/metrics.go
package main

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    architectureViolations = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "boxo_architecture_violations_total",
            Help: "Total number of architecture violations detected",
        },
        []string{"component", "violation_type", "severity"},
    )
    
    architectureCompliance = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "boxo_architecture_compliance_ratio",
            Help: "Ratio of code that complies with architecture (0-1)",
        },
        []string{"component"},
    )
    
    diagramSyncStatus = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "boxo_diagram_sync_status",
            Help: "Status of diagram synchronization with code (1=synced, 0=out of sync)",
        },
        []string{"diagram_type"},
    )
)

type ArchitectureMetricsCollector struct {
    validator *ArchitectureValidator
}

func (amc *ArchitectureMetricsCollector) CollectMetrics() error {
    violations, err := amc.validator.ValidateAll()
    if err != nil {
        return err
    }
    
    // Сброс метрик
    architectureViolations.Reset()
    
    // Подсчет нарушений по типам
    violationCounts := make(map[string]map[string]int)
    for _, violation := range violations {
        if violationCounts[violation.Component] == nil {
            violationCounts[violation.Component] = make(map[string]int)
        }
        violationCounts[violation.Component][violation.Type]++
        
        architectureViolations.WithLabelValues(
            violation.Component,
            violation.Type,
            violation.Severity,
        ).Inc()
    }
    
    // Расчет коэффициента соответствия
    for component, counts := range violationCounts {
        totalChecks := amc.validator.GetTotalChecks(component)
        totalViolations := 0
        for _, count := range counts {
            totalViolations += count
        }
        
        compliance := float64(totalChecks-totalViolations) / float64(totalChecks)
        architectureCompliance.WithLabelValues(component).Set(compliance)
    }
    
    return nil
}
```

### Grafana Dashboard для архитектурных метрик

```json
{
  "dashboard": {
    "title": "Boxo Architecture Compliance",
    "panels": [
      {
        "title": "Architecture Violations by Component",
        "type": "stat",
        "targets": [
          {
            "expr": "sum by (component) (boxo_architecture_violations_total)",
            "legendFormat": "{{component}}"
          }
        ],
        "fieldConfig": {
          "defaults": {
            "color": {
              "mode": "thresholds"
            },
            "thresholds": {
              "steps": [
                {"color": "green", "value": 0},
                {"color": "yellow", "value": 5},
                {"color": "red", "value": 10}
              ]
            }
          }
        }
      },
      {
        "title": "Architecture Compliance Ratio",
        "type": "gauge",
        "targets": [
          {
            "expr": "avg(boxo_architecture_compliance_ratio)",
            "legendFormat": "Overall Compliance"
          }
        ],
        "fieldConfig": {
          "defaults": {
            "min": 0,
            "max": 1,
            "unit": "percentunit",
            "thresholds": {
              "steps": [
                {"color": "red", "value": 0},
                {"color": "yellow", "value": 0.8},
                {"color": "green", "value": 0.95}
              ]
            }
          }
        }
      },
      {
        "title": "Violations Over Time",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(boxo_architecture_violations_total[5m])",
            "legendFormat": "{{component}} - {{violation_type}}"
          }
        ]
      }
    ]
  }
}
```

## 🔧 Development Tools

### Pre-commit Hook для валидации архитектуры

```bash
#!/bin/bash
# .git/hooks/pre-commit

echo "🏗️  Validating architecture compliance..."

# Проверка синтаксиса диаграмм
cd docs/Task5/Arch
for file in *.puml; do
    if ! plantuml -checkonly "$file" > /dev/null 2>&1; then
        echo "❌ Diagram syntax error in $file"
        exit 1
    fi
done

# Валидация архитектуры для измененных Go файлов
cd ../../../
changed_files=$(git diff --cached --name-only --diff-filter=ACM | grep '\.go$')

if [ -n "$changed_files" ]; then
    echo "Validating changed Go files..."
    
    # Сборка валидатора если необходимо
    if [ ! -f tools/arch-validator/arch-validator ]; then
        echo "Building architecture validator..."
        cd tools/arch-validator
        go build -o arch-validator .
        cd ../..
    fi
    
    # Запуск валидации
    if ! ./tools/arch-validator/arch-validator \
        --diagrams-path docs/Task5/Arch \
        --code-path bitswap/monitoring \
        --changed-files "$changed_files" \
        --output-format console; then
        echo "❌ Architecture validation failed"
        echo "Please fix violations or update architecture diagrams"
        exit 1
    fi
fi

echo "✅ Architecture validation passed"
```

### Makefile для архитектурных операций

```makefile
# Makefile для работы с архитектурой
.PHONY: arch-validate arch-generate arch-docs arch-metrics arch-clean

# Валидация архитектуры
arch-validate:
	@echo "🔍 Validating architecture..."
	@cd docs/Task5/Arch && make validate
	@cd tools/arch-validator && go run . \
		--diagrams-path ../../docs/Task5/Arch \
		--code-path ../../bitswap/monitoring \
		--output-format console

# Генерация кода из диаграмм
arch-generate:
	@echo "🏗️  Generating code from diagrams..."
	@cd tools/code-generator && go run . \
		--diagrams-path ../../docs/Task5/Arch \
		--output-path ../../bitswap/monitoring/generated \
		--template-path templates

# Генерация документации
arch-docs:
	@echo "📚 Generating architecture documentation..."
	@cd docs/Task5/Arch && make png
	@cd tools/doc-generator && go run . \
		--diagrams-path ../../docs/Task5/Arch \
		--output-path ../../docs/architecture.html

# Сбор метрик архитектуры
arch-metrics:
	@echo "📊 Collecting architecture metrics..."
	@cd tools/arch-metrics && go run . \
		--diagrams-path ../../docs/Task5/Arch \
		--code-path ../../bitswap/monitoring \
		--output-format prometheus \
		--output-file ../../metrics/architecture.prom

# Очистка сгенерированных файлов
arch-clean:
	@echo "🧹 Cleaning generated files..."
	@rm -rf bitswap/monitoring/generated/*
	@rm -f docs/Task5/Arch/*.png
	@rm -f docs/Task5/Arch/*.svg
	@rm -f docs/architecture.html
	@rm -f metrics/architecture.prom

# Полный цикл проверки архитектуры
arch-check: arch-validate arch-generate arch-docs arch-metrics
	@echo "✅ Architecture check completed"

# Настройка pre-commit hook
setup-hooks:
	@echo "🔧 Setting up git hooks..."
	@cp scripts/pre-commit .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "✅ Git hooks installed"
```

### Docker для изолированной валидации

```dockerfile
# tools/arch-validator/Dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o arch-validator .

FROM openjdk:11-jre-slim

# Установка PlantUML
RUN apt-get update && apt-get install -y wget && \
    wget -O /usr/local/bin/plantuml.jar \
    https://github.com/plantuml/plantuml/releases/latest/download/plantuml-1.2024.0.jar && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

# Копирование валидатора
COPY --from=builder /app/arch-validator /usr/local/bin/

# Создание скрипта запуска
RUN echo '#!/bin/bash\njava -jar /usr/local/bin/plantuml.jar "$@"' > /usr/local/bin/plantuml && \
    chmod +x /usr/local/bin/plantuml

WORKDIR /workspace

ENTRYPOINT ["arch-validator"]
```

```yaml
# docker-compose.yml для архитектурной валидации
version: '3.8'

services:
  arch-validator:
    build: ./tools/arch-validator
    volumes:
      - .:/workspace
    command: >
      --diagrams-path docs/Task5/Arch
      --code-path bitswap/monitoring
      --output-format json
      --output-file /workspace/violations.json
    
  diagram-generator:
    image: plantuml/plantuml-server:jetty
    ports:
      - "8080:8080"
    environment:
      - PLANTUML_LIMIT_SIZE=8192
    
  docs-server:
    image: nginx:alpine
    ports:
      - "8081:80"
    volumes:
      - ./docs:/usr/share/nginx/html:ro
```

Эта интеграция с инструментами разработки обеспечивает:

1. **Непрерывную валидацию** архитектуры на всех этапах разработки
2. **Автоматическую генерацию** кода из диаграмм
3. **Визуализацию** соответствия архитектуре через метрики
4. **Интеграцию** с популярными IDE и CI/CD системами
5. **Автоматизацию** рутинных задач через Makefile и скрипты

Это превращает архитектурные диаграммы из статической документации в живой инструмент разработки.