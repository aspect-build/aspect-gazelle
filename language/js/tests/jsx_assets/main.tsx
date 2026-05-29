// Exercises every JSX asset tag (img|video|source|audio|track) and both
// asset attributes (src|poster), in both self-closing and opening-element
// forms, to cover the asset-tag/attribute lists in the JS parser.

// img, self-closing element + src attribute
export const Img = () => <img src="./img.png" />

// video, opening element + both src and poster attributes
export const Video = () => (
  <video src="./video.mp4" poster="./video-poster.png">
    {/* source, self-closing element nested in an opening element */}
    <source src="./src.webm" />
    {/* track, self-closing element */}
    <track src="./track.vtt" />
  </video>
)

// audio, opening element + src attribute
export const Audio = () => <audio src="./audio.mp3"></audio>

// A second self-closing img to cover the self-closing form for src
export const Logo = () => <img src="./selfclose.png" />
