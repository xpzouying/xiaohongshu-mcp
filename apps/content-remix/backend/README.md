# ContentRemixAgent Backend

FastAPI + LangGraph backend for remix analysis workflows.

## Run locally

```bash
source "$(conda info --base)/etc/profile.d/conda.sh"
conda activate nanobot
pip install -r apps/content-remix/backend/requirements.txt
uvicorn app.main:app --app-dir apps/content-remix/backend --reload --port 18061
```

## Run tests

```bash
source "$(conda info --base)/etc/profile.d/conda.sh"
conda activate nanobot
pip install -r apps/content-remix/backend/requirements.txt
pip install -e "apps/content-remix/backend[test]"
pytest apps/content-remix/backend/tests -v
```

