class Danmu {
    constructor(canvas) {
        this.danmuList = []
        this.canvas = canvas
        this.lineHeight = 24;
        this.render()
    }

    setup() {
        this.canvas.width = this.canvas.clientWidth
        this.canvas.height = this.canvas.clientHeight
        this.context = this.canvas.getContext('2d')
        this.context.font = '24px Arial'
    }

    fire(msg) {
        if (this.danmuList.length > 50) {
            return
        }
        let height = this.lineHeight
        let entering = _.filter(this.danmuList, (d) => {
            return d.left + d.width > this.canvas.width
        })
        while (_.find(entering, (d) => { return d.height == height })) {
            height += this.lineHeight
        }
        this.danmuList.push({
            'value': msg,
            'left': this.canvas.width,
            'width': this.context.measureText(msg).width,
            'height': height,
            'speed': 2
        })
    }

    render() {
        this.setup()
        this.context.clearRect(0, 0, this.canvas.width, this.canvas.height)
        _.each(this.danmuList, (d) => {
            this.context.fillText(d.value, d.left, d.height)
            d.left -= d.speed
        })
        _.remove(this.danmuList, (d) => {
            return d.left + d.width < 0
        })
        requestAnimationFrame(this.render.bind(this));
    }
}
