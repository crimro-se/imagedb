from flask import Flask, request, jsonify
from multiprocessing import Queue, Process, Manager
import base64
import time
import uuid
import torch
from transformers import CLIPProcessor, CLIPModel
from PIL import Image
import io

app = Flask(__name__)
device = torch.device('cuda' if torch.cuda.is_available() else 'cpu')
model = CLIPModel.from_pretrained("openai/clip-vit-base-patch32").to(device)
processor = CLIPProcessor.from_pretrained("openai/clip-vit-base-patch32")



@app.route('/process_image', methods=['POST'])
def process_image():
    # Generate a unique ID for the request
    request_id = str(uuid.uuid4())

    # Get the image from the request
    image_data = request.json.get('image')

    # Put the request in the request queue
    request_queue.put((request_id, image_data))

    return "Cheers"



def worker_process(request_queue):
    # Initialize the CLIP model and processor
    
    while True:
        # Process requests in batches
        requests = []
        while not request_queue.empty() and len(requests) < 8:  # Batch size of 8
            request_id, image_data = request_queue.get()
            print(request_id)
            requests.append((request_id, image_data))

        if requests:
            images = []
            ids = []
            for request_id, image_data in requests:
                image = Image.open(io.BytesIO(base64.b64decode(image_data)))
                images.append(image)
                ids.append(request_id)

            # Process images and generate embeddings
            inputs = processor(images=images, return_tensors="pt", padding=True).to(device)
            with torch.no_grad():
                embeddings = model.get_image_features(**inputs)

            # Normalize embeddings
            #embeddings = embeddings / embeddings.norm(dim=-1, keepdim=True)

            # Send the results back to the API server
            for idx, embedding in enumerate(embeddings):
                result = {'embedding': embedding.cpu().tolist()}
                print(result)

# Run the API server in a separate process
#api_process = Process(target=api_server, args=(request_queue,))
#api_process.start()

# Run the worker process in a separate process
#worker = Process(target=worker_process, args=(request_queue,))
#worker.start()

#api_process.join()

if __name__ == "main":
    app.run(port=5000)