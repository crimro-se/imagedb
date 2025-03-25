import litserve as ls
import torch
import base64
import uuid
from PIL import Image
import io
from transformers import CLIPProcessor, CLIPModel
import pytorch_lightning as pl
from safetensors.torch import load_file
import torch.nn as nn

BATCH_SIZE = 24  # Define the batch size

# Define the MLP model for aesthetic scoring
class MLP(pl.LightningModule):
    def __init__(self, input_size):
        super().__init__()
        self.layers = nn.Sequential(
            nn.Linear(input_size, 1024),
            nn.Dropout(0.2),
            nn.Linear(1024, 128),
            nn.Dropout(0.2),
            nn.Linear(128, 64),
            nn.Dropout(0.1),
            nn.Linear(64, 16),
            nn.Linear(16, 1)
        )

    def forward(self, x):
        return self.layers(x)

# Normalization function for tensors
def normalized_pt(a: torch.Tensor, axis: int = -1, order: int = 2) -> torch.Tensor:
    norms = torch.norm(a, order, dim=axis, keepdim=True)
    norms[norms == 0] = 1  # Avoid division by zero
    return a / norms

# LitServe API definition
class ClipLitAPI(ls.LitAPI):
    def setup(self, device):
        self.device = device
        
        # Load CLIP model and processor
        self.model = CLIPModel.from_pretrained(
            "openai/clip-vit-large-patch14",
            torch_dtype=torch.float16
        ).to(device, torch.float16)
        self.processor = CLIPProcessor.from_pretrained("openai/clip-vit-large-patch14")
        
        # Load aesthetic model
        self.aesthetic = MLP(768)
        state_dict = load_file("sac+logos+ava1-l14-linearMSE.safetensors")
        self.aesthetic.load_state_dict(state_dict)
        self.aesthetic.to(device, dtype=torch.float16)
        
        # Compile models for performance
        self.model = torch.compile(self.model)
        self.aesthetic = torch.compile(self.aesthetic)

    def decode_request(self, request):
        # Handle batch requests
        if isinstance(request, list):
            print(f"Received a list instead of a dictionary: {request}")
            raise ValueError("Unexpected list received")

        return self._decode_single(request)
    
    def _decode_single(self, item):
        # This is actually more like a request validator
        if isinstance(item, list):
            print(f"Received a list instead of a dictionary: {item}")
            raise ValueError("Unexpected list received")
    
        task_id = item.get("id", str(uuid.uuid4()))
        image = item.get("image")
        text = item.get("text")
        
        if image and text:
            raise ValueError("Submit either image or text, not both")
        if not image and not text:
            raise ValueError("Either image or text is required")
        
        return {"id": task_id, "image": image, "text": text}

    def predict(self, batch):
        # Split batch into images and texts
        image_indices, images = [], []
        text_indices, texts = [], []
        
        for idx, item in enumerate(batch):
            #print(item)
            if item.get("image"):
                image_indices.append(idx)
                images.append(item["image"])
            else:
                text_indices.append(idx)
                texts.append(item["text"].strip())
        
        results = [None] * len(batch)
        
        # Process images
        if images:
            pil_images = []
            for img_b64 in images:
                img_bytes = base64.b64decode(img_b64)
                pil_images.append(Image.open(io.BytesIO(img_bytes)))
            
            inputs = self.processor(images=pil_images, return_tensors="pt").to(self.device)
            with torch.no_grad():
                image_features = self.model.get_image_features(**inputs)
                image_features = normalized_pt(image_features)
                aesthetic_scores = self.aesthetic(image_features).squeeze(-1).tolist()
                embeddings = image_features.cpu().tolist()
            
            for i, idx in enumerate(image_indices):
                results[idx] = {
                    "id": batch[idx]["id"],
                    "embedding": embeddings[i],
                    "aesthetic": aesthetic_scores[i]
                }
        
        # Process texts
        if texts:
            inputs = self.processor(
                text=texts,
                return_tensors="pt",
                padding=True,
                truncation=True
            ).to(self.device)
            with torch.no_grad():
                text_features = self.model.get_text_features(**inputs)
                text_features = normalized_pt(text_features)
                embeddings = text_features.cpu().tolist()
            
            for i, idx in enumerate(text_indices):
                results[idx] = {
                    "id": batch[idx]["id"],
                    "embedding": embeddings[i],
                    "aesthetic": 0.0  # No aesthetic score for text
                }
        
        return results

    def encode_response(self, output):
        return output

if __name__ == "__main__":
    api = ClipLitAPI()
    server = ls.LitServer(api, accelerator="cuda", devices=1, max_batch_size=BATCH_SIZE, workers_per_device=4)
    server.run(port=5000)