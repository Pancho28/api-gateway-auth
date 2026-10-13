from fastapi import FastAPI, Header, HTTPException, status

app = FastAPI()

# Simulación de la variable de entorno obligada por el agent.md
GATEWAY_SECRET_REQUIRED = "8194e51526ee6323c1fe793cf705694fcae30becf802a714f0b1e9f7e451b2f4"

@app.get("/inventory/{item_id}")
async def get_stock(
    item_id: int, 
    x_internal_api_key: str = Header(None)
):
    """
    Endpoint que valida la API Key interna para el servicio de Stock.
    """
    if x_internal_api_key != GATEWAY_SECRET_REQUIRED:
        raise HTTPException(
            status_code=status.HTTP_403_FORBIDDEN,
            detail="Acceso prohibido: API Key interna de Stock inválida"
        )
        
    return {
        "message": "Respuesta desde el microservicio de Stock (Protegido)", 
        "item_id": item_id, 
        "stock": 150,
        "status": "available"
    }
