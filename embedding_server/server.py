from flask import Flask, request, jsonify
from multiprocessing import Queue, Process, Manager
from queue import Empty
import base64
import time
import uuid
import torch
import torch.nn as nn
import multiprocessing as mp
from transformers import CLIPProcessor, CLIPModel
from PIL import Image
import io
import pytorch_lightning as pl
from safetensors.torch import load_file

BATCH_SIZE = 10  # Define the batch size

# from christophschuhmann/improved-aesthetic-predictor
class MLP(pl.LightningModule):
    def __init__(self, input_size, xcol='emb', ycol='avg_rating'):
        super().__init__()
        self.input_size = input_size
        self.xcol = xcol
        self.ycol = ycol
        self.layers = nn.Sequential(
            nn.Linear(self.input_size, 1024),
            #nn.ReLU(),
            nn.Dropout(0.2),
            nn.Linear(1024, 128),
            #nn.ReLU(),
            nn.Dropout(0.2),
            nn.Linear(128, 64),
            #nn.ReLU(),
            nn.Dropout(0.1),

            nn.Linear(64, 16),
            #nn.ReLU(),

            nn.Linear(16, 1)
        )

    def forward(self, x):
        return self.layers(x)

    def training_step(self, batch, batch_idx):
            x = batch[self.xcol]
            y = batch[self.ycol].reshape(-1, 1)
            x_hat = self.layers(x)
            loss = F.mse_loss(x_hat, y)
            return loss
    
    def validation_step(self, batch, batch_idx):
        x = batch[self.xcol]
        y = batch[self.ycol].reshape(-1, 1)
        x_hat = self.layers(x)
        loss = F.mse_loss(x_hat, y)
        return loss

    def configure_optimizers(self):
        optimizer = torch.optim.Adam(self.parameters(), lr=1e-3)
        return optimizer

def normalized(a, axis=-1, order=2):
    import numpy as np  # pylint: disable=import-outside-toplevel

    l2 = np.atleast_1d(np.linalg.norm(a, order, axis))
    l2[l2 == 0] = 1
    return a / np.expand_dims(l2, axis)

# can you believe the original inferrence example for the
# aesthetic predictor involves converting the embedding to numpy
# just to normalize it??? Then back to a tensor??? Then back to the GPU???
def normalized_pt(a: torch.Tensor, axis: int = -1, order: int = 2) -> torch.Tensor:
    """
    Normalizes the input tensor `a` along the specified `axis` to unit vectors
    using the L2 norm (Euclidean norm) by default (order=2).
    
    Parameters:
    - `a`: Input tensor
    - `axis`: Axis along which to compute the norm (default: last axis, `-1`)
    - `order`: Order of the norm (default: `2` for L2/Euclidean norm)
    
    Returns:
    - The normalized tensor where each vector along `axis` has a length of 1.
    """
    norms = torch.norm(a, order, dim=axis, keepdim=True)
    norms[norms == 0] = 1  # Avoid division by zero
    return a / norms




def process_tasks(task_queue, results):
    device = torch.device('cuda' if torch.cuda.is_available() else 'cpu')
    model = CLIPModel.from_pretrained("openai/clip-vit-large-patch14").to(device)
    processor = CLIPProcessor.from_pretrained("openai/clip-vit-large-patch14")
    aesthetic = MLP(768)
    #s = torch.load("sac+logos+ava1-l14-linearMSE.pth")
    s = load_file("sac+logos+ava1-l14-linearMSE.safetensors")
    aesthetic.load_state_dict(s)
    aesthetic.to(device)


    while True:
        batch = []
        # we block to get a task to avoid busy looping...
        task = task_queue.get()
        if task is None:
            break # Stop signal
        batch.append(task)
        # now we fill up the batch as much as possible
        if not get_additional_tasks(batch, task_queue):
            break

        img_batch, txt_batch = [],[]
        images, texts = [],[]
        print (len(batch))
        for t in batch:
            task_id, image_data, text_data = t
            if image_data:
                img_batch.append((task_id, base64.b64decode(image_data)))
            if text_data:
                txt_batch.append((task_id, text_data))

        if img_batch:
            images = [Image.open(io.BytesIO(data[1])) for data in img_batch]
            inputs = processor(images=images, return_tensors="pt").to(device)
            with torch.no_grad():
                clip_emb = model.get_image_features(**inputs)
                emb_norm = normalized_pt(clip_emb.detach())
                aesthetic_scores = aesthetic(emb_norm).cpu().numpy().tolist()
                image_embeddings = clip_emb.cpu().numpy().tolist()
            for i, emb in enumerate(image_embeddings):
                results[img_batch[i][0]] = (emb, aesthetic_scores[i])
        
        if txt_batch:
            texts = [t[1] for t in txt_batch]
            inputs = processor(text=texts, return_tensors="pt").to(device)
            with torch.no_grad():
                text_embeddings = model.get_text_features(**inputs).cpu().numpy().tolist()
                for i, emb in enumerate(text_embeddings):
                    results[txt_batch[i][0]] = emb

def get_additional_tasks(batch, q):
    try: 
        while len(batch) < BATCH_SIZE:
            task = q.get_nowait()
            if task is None: 
                return False
            batch.append(task)
    except Empty:
        return True
    return True

if __name__ == '__main__':
    mp.set_start_method('spawn')
    # Start the task processor in the background

    app = Flask(__name__)
    # Shared Queue for Tasks
    task_queue = Queue()

    # Shared Dictionary for results
    mgr = Manager() 
    results = mgr.dict()

    processor_process = Process(target=process_tasks, args=(task_queue,results))
    processor_process.daemon = True  # So it exits when main process exits
    processor_process.start()

    @app.route('/process', methods=['POST'])
    def process_endpoint():
        data = request.get_json()
        if not data:
            return jsonify({'error': 'No JSON data received'}), 400
        
        task_id = data.get('id') or str(uuid.uuid4())
        image_data = data.get('image')
        text_data = data.get('text')
        
        if not (image_data or text_data):
            return jsonify({'error': 'Either image or text data is required'}), 400
        
        if (image_data and text_data):
            return jsonify({'error': 'Submit an image or text, not both'}), 400
        
        task_queue.put((task_id, image_data, text_data))
        return jsonify({'message': 'Task submitted successfully', 'id': task_id}), 202

    @app.route('/result/<string:id>', methods=['GET'])
    def result_endpoint(id):
        if id not in results:
            return jsonify({'error': 'Result not found or not yet processed'}), 404
        return jsonify({'id': id, 'result': results[id]}), 200
    
    try:
        app.run(debug=False)
    finally:
        task_queue.put(None)  # Stop signal
        processor_process.join()