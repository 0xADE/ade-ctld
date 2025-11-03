# CMDLIST protocol

Support binary or text representation.

General for both formats:
| bytes numbers (+len) | data           | desc                        |
| 0-2 (3)              | "TXT" or "BIN" | Text or binary format used. |

Text format:
| byte number (+len) | example | desc                      |
| 3-4 (2)            | "01"    | Protocol version in ASCII |
|                    |         |                           |

Binary format:
| byte number(+len) | example | desc                        |
| 3 (1)             | 01      | Protocol version as binary. |

## Commands

### +filter-name
Принимает произвольный список аргументов типов <str> или <bool> (OR, AND, NOT).  Устанавливает фильтр имени файлов, по которому они ищутся в PATH.
В имени рассматривается как имя файла непосредственно, так и его заголовки из desktop-файлов.

### +filter-cat
Добавить аргументы из <str> параметров как фильтры категорий по AND (если к каким-то из них не добавлен OR).

### +filter-path
Добавить аргументы из <str> параметров как фильтры путей по OR (если к каким-то из них не добавлен AND).

### 0filters
Сбросить все фильтры.

### list
Параметров нет. Вернуть список имен (с их ID), по текущему набору фильтров. Разные фильтры склеиваются по OR.

### run
Обязательный параметр: id <int>

ID из индексной базы, по которому производится запуск.

### lang
Параметр обязательный: isolang <str>

Предпочитаемый язык возврата результатов (например при выборе локализаций возращаемых из desktop-файлов)

## Fort style

Uses reverse Polish notation for commands and arguments.

- Any argument followed by 0A (ASCII LF).
- Any string arg prefixed with ", тип <str>
- Comment lines started with # and just ignored
- Пустые команды (0A следуют подряд) просто игнорируются и отражаются в листинге пропусками строк
- Логические операторы or, and, not (это не строки, передаются без префикса "), тип <bool>
- Числа передаются тоже без " (тип <int>)

### Examples

Пример сессии:
```
# Добавить данные в список путей:
"~/bin
"~/apps
+path

# Сохранить текущие настройки в файловый конфиг:
saveconf

# установить фильтрацию по именам программ по подстрокe "fi+fox" (например firefox)
"fi fox
+filter-name

# Добавить фильтр на категории graphics AND viewers (иначе по дефолту подрузумевается OR)
"graphics
"viewers
and
+filter-cat
# Вернуть список имен (заголовки для списка в UI + id), попадающих в текущий фильтр
list
# Вернуть следующую порцию из фильтра
list-next
```

```
#log залогировать эту строку, пример комментария с указанием (pragma word after # without space)
```

Команды обрабатываются по мере готовности. Аргументы просто кладутся в стек, один за одним, пока не придёт команда к ним. Если аргументы валидны для команды, она исполняется и отдаёт результат в сокет. Иначе в сокет выдается ошибка с описанием проблемы.

## Command results

Результат любой команды отдается как:

<attrs block>
[body block]

Блок attrs можно рассматривать как заголовки. Это атрибуты вида <key> <value>, где между key и value стоит один разделитель (пробел).
После value обязательно LF (0A). Блок завершается двумя LF подряд, после чего опционально идет блок с телом ответа.

Примеры ответа на команду list:

```
list-len: 2
pages: 1

1235 Firefox
1262 Firefox (Wayland)
```

Тогда после запроса от UI:
```
1262
run
```

будет произведен запуск firefox for wayland и вернется статус запуска и проверки наличия процесса:

```
cmd: run
idx: 1262
status: 0
pid: 2365
```

А вот пример ответа на ошибочный запрос:

```
0
run
```

Reply:
```
error-cmd: run
error: index not found
desc: Can't run application, requested index nof found.
```

Body is empty here, because of error.
