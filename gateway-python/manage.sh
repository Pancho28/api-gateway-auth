#!/bin/bash

# Detener el script si ocurre algún error
set -e

# Función para mostrar la ayuda
show_help() {
    echo "🛠️  Gestor de Tareas del API Gateway"
    echo "Uso: ./manage.sh [comando]"
    echo ""
    echo "Comandos disponibles:"
    echo "  gateway   🚀 Inicia el API Gateway localmente (puerto 8000)"
    echo "  mock [tipo] 🧪 Inicia mocks (ventas=8001, stock=8002)"
    echo "  tests     🧪 Ejecuta pruebas unitarias y reporte de cobertura"
    echo "  help      ❓ Muestra este menú de ayuda"
    echo ""
}

# Verificación y activación del entorno virtual
if [ -d "venv" ]; then
    # Silencioso para no ensuciar el output a menos que haya un problema
    if [ -f "venv/Scripts/activate" ]; then
        # Entorno Windows (Git Bash, WSL)
        source venv/Scripts/activate
    elif [ -f "venv/Scripts/activate" ]; then
        # Entorno Linux / macOS
        source venv/Scripts/activate
    else
        echo "⚠️ No se encontró el script de activación en la carpeta 'venv'."
    fi
else
    echo "⚠️ Advertencia: No se encontró un directorio 'venv'. El script intentará ejecutarse con el entorno global de Python."
fi

# Procesamiento de comandos con la sentencia case
case "$1" in
    gateway)
        echo "🚀 Iniciando API Gateway con recarga automática..."
        # Ajustado a 'main:app' en lugar de 'app.main:app' dado que tu archivo principal se llama main.py y está en la raíz
        uvicorn main:app --reload --port 8000
        ;;

    mock)
        if [ "$2" == "ventas" ]; then
            echo "🧪 Iniciando microservicio MOCK de Ventas en puerto 8001..."
            uvicorn debug.mock_ventas:app --port 8001 --reload
        elif [ "$2" == "stock" ]; then
            echo "🧪 Iniciando microservicio MOCK de Stock en puerto 8002..."
            uvicorn debug.mock_stock:app --port 8002 --reload
        else
            echo "❌ Error: Debes especificar qué mock iniciar (ventas o stock)."
            echo "Ejemplo: ./manage.sh mock ventas"
            echo "Ejemplo: ./manage.sh mock stock"
            exit 1
        fi
        ;;
        
    tests)
        echo "🧪 Ejecutando suite de pruebas unitarias y cobertura..."
        export PYTHONPATH=.
        pytest --cov=core --cov=api --cov=services --cov-report=term-missing tests/
        ;;
        
    help)
        show_help
        ;;
        
    *)
        # Manejo de comando no reconocido o vacío
        show_help
        if [ -n "$1" ]; then
            echo "❌ Error: Comando '$1' no reconocido."
        else
            echo "❌ Error: Debes ingresar un comando."
        fi
        exit 1
        ;;
esac
