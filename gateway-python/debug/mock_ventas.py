from fastapi import FastAPI, Header, HTTPException, status
import os

app = FastAPI()

# Simulación de la variable de entorno obligada por el agent.md
GATEWAY_SECRET_REQUIRED = "d42af192df166c118b1be94cc3223df6ae97cd3721943ac6d5eb02a2cd4b1a3b"

@app.get("/items/{item_id}")
async def read_item(
    item_id: int, 
    x_internal_api_key: str = Header(None)
):
    """
    Endpoint que valida la API Key interna.
    Si no coincide con GATEWAY_SECRET_REQUIRED, lanza 403 Forbidden.
    """
    if x_internal_api_key != GATEWAY_SECRET_REQUIRED:
        raise HTTPException(
            status_code=status.HTTP_403_FORBIDDEN,
            detail="Acceso prohibido: API Key interna inválida o ausente"
        )
        
    return {
        "message": "Respuesta desde el microservicio de Ventas (Protegido)", 
        "item_id": item_id, 
        "status": "success"
    }

@app.post("/items/")
async def create_item(
    item: dict,
    x_internal_api_key: str = Header(None)
):
    """Endpoint de creación con validación de seguridad."""
    if x_internal_api_key != GATEWAY_SECRET_REQUIRED:
        raise HTTPException(
            status_code=status.HTTP_403_FORBIDDEN,
            detail="Acceso prohibido"
        )
    return {"message": "Item creado en Ventas", "data": item}
