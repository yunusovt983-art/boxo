# Makefile для генерации архитектурных диаграмм Task 5
# Требует установленного PlantUML

PLANTUML_JAR = plantuml.jar
PLANTUML_URL = https://github.com/plantuml/plantuml/releases/latest/download/plantuml-1.2024.0.jar

# Исходные файлы диаграмм
PUML_FILES = context-diagram.puml \
             container-diagram.puml \
             component-diagram.puml \
             code-diagram.puml \
             sequence-diagram.puml \
             deployment-diagram.puml \
             state-diagram.puml \
             class-diagram.puml

# Выходные файлы
PNG_FILES = $(PUML_FILES:.puml=.png)
SVG_FILES = $(PUML_FILES:.puml=.svg)
PDF_FILES = $(PUML_FILES:.puml=.pdf)

# Цели по умолчанию
.PHONY: all clean png svg pdf install-plantuml help

all: png

help:
	@echo "Доступные команды:"
	@echo "  make png          - Генерация PNG диаграмм"
	@echo "  make svg          - Генерация SVG диаграмм"
	@echo "  make pdf          - Генерация PDF диаграмм"
	@echo "  make all          - Генерация всех PNG диаграмм (по умолчанию)"
	@echo "  make clean        - Удаление сгенерированных файлов"
	@echo "  make install-plantuml - Скачивание PlantUML JAR"
	@echo "  make validate     - Проверка синтаксиса всех диаграмм"

# Установка PlantUML
install-plantuml:
	@echo "Скачивание PlantUML..."
	curl -L -o $(PLANTUML_JAR) $(PLANTUML_URL)
	@echo "PlantUML установлен как $(PLANTUML_JAR)"

# Проверка наличия PlantUML
check-plantuml:
	@if [ ! -f $(PLANTUML_JAR) ]; then \
		echo "PlantUML не найден. Запустите 'make install-plantuml'"; \
		exit 1; \
	fi

# Генерация PNG файлов
png: check-plantuml $(PNG_FILES)

%.png: %.puml
	@echo "Генерация $@..."
	java -jar $(PLANTUML_JAR) -tpng $<

# Генерация SVG файлов
svg: check-plantuml $(SVG_FILES)

%.svg: %.puml
	@echo "Генерация $@..."
	java -jar $(PLANTUML_JAR) -tsvg $<

# Генерация PDF файлов
pdf: check-plantuml $(PDF_FILES)

%.pdf: %.puml
	@echo "Генерация $@..."
	java -jar $(PLANTUML_JAR) -tpdf $<

# Проверка синтаксиса
validate: check-plantuml
	@echo "Проверка синтаксиса диаграмм..."
	@for file in $(PUML_FILES); do \
		echo "Проверка $$file..."; \
		java -jar $(PLANTUML_JAR) -checkonly $$file || exit 1; \
	done
	@echo "Все диаграммы прошли проверку синтаксиса!"

# Генерация всех форматов
all-formats: png svg pdf

# Очистка сгенерированных файлов
clean:
	@echo "Удаление сгенерированных файлов..."
	rm -f $(PNG_FILES) $(SVG_FILES) $(PDF_FILES)
	@echo "Очистка завершена"

# Создание архива с диаграммами
archive: png
	@echo "Создание архива диаграмм..."
	tar -czf task5-architecture-diagrams.tar.gz *.png *.puml README.md index.html
	@echo "Архив создан: task5-architecture-diagrams.tar.gz"

# Запуск локального веб-сервера для просмотра
serve:
	@echo "Запуск локального веб-сервера на порту 8080..."
	@echo "Откройте http://localhost:8080 в браузере"
	python3 -m http.server 8080 2>/dev/null || python -m SimpleHTTPServer 8080

# Автоматическая генерация при изменении файлов (требует inotify-tools)
watch:
	@echo "Отслеживание изменений в .puml файлах..."
	@echo "Нажмите Ctrl+C для остановки"
	while inotifywait -e modify *.puml; do make png; done

# Статистика по диаграммам
stats:
	@echo "Статистика по диаграммам:"
	@echo "Количество .puml файлов: $(words $(PUML_FILES))"
	@echo "Общий размер .puml файлов: $$(du -ch *.puml | tail -1 | cut -f1)"
	@if ls *.png >/dev/null 2>&1; then \
		echo "Количество .png файлов: $$(ls *.png | wc -l)"; \
		echo "Общий размер .png файлов: $$(du -ch *.png 2>/dev/null | tail -1 | cut -f1)"; \
	fi

# Проверка зависимостей
check-deps:
	@echo "Проверка зависимостей..."
	@command -v java >/dev/null 2>&1 || { echo "Java не установлена"; exit 1; }
	@command -v curl >/dev/null 2>&1 || { echo "curl не установлен"; exit 1; }
	@echo "Все зависимости установлены"

# Обновление PlantUML до последней версии
update-plantuml:
	@echo "Обновление PlantUML до последней версии..."
	rm -f $(PLANTUML_JAR)
	$(MAKE) install-plantuml

# Создание превью всех диаграмм в одном HTML файле
preview: png
	@echo "Создание превью диаграмм..."
	@echo '<!DOCTYPE html><html><head><title>Task 5 Architecture Preview</title></head><body>' > preview.html
	@echo '<h1>Task 5: Система мониторинга производительности - Превью диаграмм</h1>' >> preview.html
	@for png in $(PNG_FILES); do \
		echo "<h2>$$png</h2><img src=\"$$png\" style=\"max-width:100%;height:auto;\"><hr>" >> preview.html; \
	done
	@echo '</body></html>' >> preview.html
	@echo "Превью создано: preview.html"

.PHONY: check-plantuml validate all-formats archive serve watch stats check-deps update-plantuml preview